# Implementation plan: Riglight milestone 1

Status: Milestone 1 implemented, independently reviewed, and integrated with
GPT-only Herdr workers and isolated feature worktrees. The full browser gate
remains partially blocked; see `../docs/verification.md`.
Scope: the approved CPU/RAM milestone in `docs/spec.md`, not the later GPU,
llama-swap, Docker, or deployment integrations.
Baseline: initial documentation preserved in commit `02372a6`.

## Delivery strategy

First prove the high-risk stream lifecycle and browser security boundary using
synthetic data. Then connect a minimal UI, add real metrics, and polish charts
and recovery states. One writer per slice; test and checkpoint each increment.
Implementation is authorized. Tooling may be installed through the project-local
mise configuration; modifying system services remains outside scope.

Go serves a static React frontend and the versioned API in production. Use
standard `net/http`, no router framework, database, event-bus framework, auth
provider, external assets, or chart dependency. Two small SVG sparklines are
sufficient for this milestone. The application module may initially be named
`riglight` without asserting ownership of a public module domain.

## Reviewed version baseline

Checked against official registries and upstream repositories on 2026-10-18.
These are proposed exact pins, not installed dependencies. Recheck advisories
and resolve transitive dependencies at scaffolding; never silently upgrade.

| Component | Proposed pin |
| --- | --- |
| Go | 1.27.2 |
| Node, LTS | 24.22.0 |
| pnpm | 12.14.0 |
| gopsutil/v4 | v4.26.9 |
| govulncheck, golang.org/x/vuln | v1.9.0 |
| Gitleaks | v8.30.1 |
| react, react-dom | 19.3.1 |
| vite | 8.3.4 |
| @vitejs/plugin-react | 6.1.2 |
| typescript | 6.0.3 |
| @types/node | 24.19.2 |
| @types/react, @types/react-dom | 19.3.0 |
| eslint | 10.13.0 |
| @eslint/js | 10.0.1 |
| typescript-eslint | 8.71.2 |
| eslint-plugin-react-hooks | 7.1.1 |
| eslint-plugin-react-refresh | 0.5.7 |
| globals | 17.13.0 |
| vitest | 5.0.5 |
| jsdom | 30.1.3 |
| @testing-library/react | 16.3.3 |
| @testing-library/dom | 10.4.2 |
| @testing-library/jest-dom | 7.0.1 |
| @playwright/test | 1.64.1 |

TypeScript 7.0.2 is currently latest but typescript-eslint 8.71.2 declares
`typescript >=4.8.4 <6.1.0`; select 6.0.3 rather than ignoring peer warnings.
Node 24.22.0 meets the inspected Vite, Vitest, jsdom, and ESLint engine ranges.
React and react-dom match. Additional React compiler/Babel peers of the Vite
plugin are optional; do not enable React Compiler or add those peers here.

Health check: React, Vite, Vitest, TypeScript, Playwright, gopsutil, and Gitleaks
are active, non-archived projects with substantial adoption; repository activity
was checked. Gopsutil's GitHub license detector returned NOASSERTION, so its
actual LICENSE was inspected: BSD-3-Clause-style terms and a Go-derived notice.
Preserve all required upstream notices when distributing. This is an initial
health check, not proof of supply-chain safety; inspect the final dependency
and install-script graph, native binaries, and advisories before use.

Frontend installation boundary: `web/`, with its own package.json,
pnpm-workspace.yaml script policy, and pnpm-lock.yaml. No root frontend lockfile.
Begin installs with `--ignore-scripts`; inspect package scripts before enabling
any named dependency build. Never blanket-allow scripts. Pin the package manager
in package.json and tools in root mise.toml; do not change global tool versions.
Use a supported local C compiler for Go race checking; disclose if unavailable.

## Contract and data flow

```text
single collector -> immutable latest sample -> bounded subscriber queues
                           |                         |
                     GET snapshot             GET SSE snapshot
                                                     |
                                        one EventSource per dashboard
```

- The browser consumes its initial sample from SSE, not a competing snapshot
  fetch. `GET /api/v1/snapshot` remains useful for API consumers and tests.
- Subscribe and enqueue the current sample atomically under the same lock used
  by publication so a new subscriber cannot receive an older initial sample.
- Each subscriber has a capacity-one latest-sample queue. Serialize replacement
  safely; publish never waits for a network client. Disconnect and shutdown
  unsubscribe exactly once. Do not close channels while publishers can send.
- Limit to eight SSE clients, returning 503 with a generic error when full.
  Browser per-origin HTTP/1 connection limits may be lower; eight is a server
  limit, not a guarantee that one browser can stream eight tabs concurrently.
- Send `event: snapshot`, JSON `data:`, a blank terminator, and `retry: 2000`.
  Flush each event. No event IDs, replay, or unbounded backlog.
- Disable an overall response WriteTimeout for SSE and enforce a five-second
  write/flush deadline per frame with ResponseController. Bound request headers,
  ReadHeaderTimeout and idle connections. Use context cancellation and a bounded
  shutdown; stream handlers must exit when the application context is cancelled.
- Every two seconds, sample CPU/RAM once; startup has NOT_READY states until
  measurements exist. CPU needs a baseline before reporting interval usage.
- Capture CPU time counters and compute aggregate deltas between collection
  ticks rather than relying on a package-global percentage cache. Verify Linux
  counter/guest/idle accounting against gopsutil source and tests before coding.
  Use VirtualMemoryWithContext for memory. No host identity or process queries.
- Collection failures publish unavailable/null values, never fabricated zeroes
  or raw error strings. Validate finite CPU percentages and sane byte totals.

### Sample schema

One JSON shape for snapshot response and SSE data; camelCase fields, UTC RFC3339
sample timestamps, uppercase status/mode enums, bytes as numbers, percent 0–100.
Browser validation must reject nonfinite/out-of-range/unsafe-integer numbers.
Do not serialize whole gopsutil structs.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "cpu": {
    "status": "AVAILABLE",
    "sampledAt": "2026-01-01T00:00:02Z",
    "ageMs": 0,
    "utilizationPercent": 42.5
  },
  "memory": {
    "status": "AVAILABLE",
    "sampledAt": "2026-01-01T00:00:02Z",
    "ageMs": 0,
    "usedBytes": 8589934592,
    "totalBytes": 17179869184
  }
}
```

Mode is DEMO or LIVE. Metric status is NOT_READY, AVAILABLE, or UNAVAILABLE.
For a non-AVAILABLE metric, its sampledAt, ageMs, and measurements are null.
For AVAILABLE metrics, ageMs is a finite, nonnegative safe-integer number of
milliseconds since collection, computed by the server at snapshot/SSE frame
serialization from its retained monotonic collection timestamp. Do not cache a
wire age at publication or derive it from browser time or reparsed wall time.
Envelope sampledAt records the attempt, not a promise of successful measurement.
Errors are `{ "error": { "code": "...", "message": "..." } }` with stable generic
codes: INVALID_REQUEST (400), FORBIDDEN (403), NOT_FOUND (404),
METHOD_NOT_ALLOWED (405, with Allow), STREAM_LIMIT (503), INTERNAL_ERROR (500).
`GET /healthz` returns `{ "status": "OK" }`; it does not certify collector health.
Invalid host/origin requests fail before collection/subscription. Unsupported
methods cannot open streams. No request body, arbitrary upstream URL, or query
parameter is required by any milestone-1 endpoint.

## Browser behavior

- Use one EventSource in an effect with full cleanup; React Strict Mode's extra
  setup/cleanup cycle must not leak connections, timers, or listeners.
- onopen indicates transport connectivity only, not fresh metric data.
  onerror shows reconnecting; native EventSource handles reconnect attempts.
- Validate schema/version before accepting a frame. Unknown/invalid frames do
  not advance freshness. A failed metric is visibly unavailable immediately.
- Track per-metric freshness as server ageMs at receipt plus monotonic browser
  elapsed time since receipt. Use sampledAt for display/deduplication, never
  subtract it from the browser wall clock. Replayed identical timestamps may
  shorten but never extend the current expiry. A cached initial/reconnect frame
  already six seconds old is immediately stale. Test browser clock skew in both
  directions and server-monotonic sample age increasing without publication.
- After six seconds without fresh valid metric data, mark its prior value stale.
  A lack of any successful value displays unavailable/pending, not zero.
- Append once per distinct sample, maintain at most 450 samples, and represent
  unavailable intervals as gaps. Never bridge gaps with an apparently healthy
  continuous sparkline. Keep state in memory only.
- Clearly show DEMO versus LIVE, text status labels, last-update information,
  keyboard focus, responsive layout, and reduced-motion behavior. No rapidly
  changing screen-reader announcements for every two-second metric update.

## HTTP and build security

- Bind to loopback only for this milestone; reject wildcard/non-loopback listen
  addresses until a separate access-control/deployment design is approved.
- Default 127.0.0.1:9090. Match exact configured Host authorities, not suffixes;
  reject malformed/multiple/foreign Origin values and Origin: null. Missing
  Origin is allowed for curl and same-origin GET clients after Host validation.
- Do not trust Forwarded/X-Forwarded-* or implement wildcard CORS. Vite dev mode
  also binds loopback; document its exact allowed local origin in dev config
  rather than turning off the backend checks for development.
- Set CSP allowing local assets only, nosniff, frame-ancestors none, no-referrer,
  and no-store for API/stream responses. HTTPS proxy owns HSTS later.
- Build frontend assets before Go embed; embed web/dist through a small Go
  package under web/. No path-traversal access, directory listings, source maps,
  arbitrary filesystem fallback, or API-to-index.html fallback.
- `go build -trimpath` for distributable binaries; validate the artifact does not
  contain local checkout paths. No identifying build metadata or .env in assets.
- Demo mode selects a synthetic collector before any live collector is created;
  test that selecting demo never performs host collection.
- Git author attribution intentionally uses the existing configured identity,
  as explicitly chosen by the owner. Content/runtime privacy does not imply
  anonymized Git authorship.

## Verification and publication gate

Implement the commands in the spec; run those applicable after every slice.
Final order: frozen frontend install, frontend tests/lint/typecheck/build, Go
format/race/vet/build, Chromium/WebKit demo smoke tests, dependency audits,
Gitleaks scan, artifact privacy inspection, independent review, and reverify any
accepted fixes. Do not report checks as passed if their tools are unavailable.

Exact additional commands after tool installation/scaffolding:

```sh
mise exec -- go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
pnpm --dir web exec playwright install chromium webkit
gitleaks git --redact --no-banner .
gitleaks dir --redact --no-banner .
git diff --check
```

Install Gitleaks v8.30.1 using verified release checksums; do not download and
execute an unverified installer. Browser binary downloads are developer tooling,
not runtime assets. If browser system libraries are missing, report that rather
than running privileged package installation. Audits contact public advisory
services; do not upload application logs, local config, or test artifacts.

Gitleaks does not detect all personal information. Also review tracked files,
history, screenshot/fixture provenance, and distributable artifact paths.
Do not push, create a remote repository, configure ingress, enable a service,
or change Docker permissions without explicit permission.

Independent review: separate GPT worker through openai-codex, as explicitly
required by the owner for this project. No local or non-GPT workers. Choose a
separate context/model from the implementer where possible; high-risk
security/concurrency review uses high reasoning. Coordinator triages evidence;
one writer fixes accepted findings.

## Dependencies and checkpoints

See `todo.md` for bounded tasks. Contracts precede consumers. Synthetic transport
and security tests precede live collection. UI and server work can be delegated
separately only once the sample contract is fixed and files are exclusive.
Checkpoints after small groups of tasks provide runnable feature demonstrations
and human review opportunities. Do not implement later integrations speculatively.

## Risks

| Risk | Mitigation |
| --- | --- |
| SSE deadlocks/leaks/slow clients | Atomic subscribe; bounded queues/deadlines; race and disconnect tests |
| Misleading CPU/memory/freshness | Verified counters; explicit null states; sample age/skew tests |
| Browser attacks on a local service | Exact Host/Origin checks; no wildcard CORS; loopback only |
| Public artifact data leaks | Synthetic demo; allowlists; no maps; trimpath; history/artifact review |
| Dependency/native script compromise | Exact pins; lockfile review; scripts blocked; advisories/notices |
| Current packages incompatible | Peer/engine validation; TS 6 pin; clean-install checks |
| Missing browser/compiler/reviewer | Explicit blockers; no privileged install or false pass |

## Sources inspected

- Versions: https://go.dev/dl/?mode=json and https://nodejs.org/dist/index.json
- Go modules: https://proxy.golang.org/github.com/shirou/gopsutil/v4/@latest and
  https://proxy.golang.org/golang.org/x/vuln/@latest
- Frontend versions/peer ranges: official https://registry.npmjs.org package
  manifests; TypeScript 6.0.3 and Node-24 type pins resolved from version metadata.
- Host metrics: https://pkg.go.dev/github.com/shirou/gopsutil/v4/cpu and
  https://pkg.go.dev/github.com/shirou/gopsutil/v4/mem
- Stream deadlines: https://pkg.go.dev/net/http#ResponseController.SetWriteDeadline
- SSE: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events
- Effect cleanup: https://react.dev/reference/react/useEffect
- Vite runtime requirements: https://vite.dev/guide/
- Dependency policy entrypoint: https://pnpm.io/settings (follow the installed
  version's dependency-script settings during scaffolding, not outdated recipes).
- Browser tooling: https://playwright.dev/docs/browsers
- Upstream repository health, license, and Gitleaks release metadata: GitHub API
  via gh, including direct inspection of gopsutil/LICENSE.

## Planning verification

Passed: Markdown fences and local links, targeted privacy/credential checks,
ignore-rule checks, whitespace checks, and validation that all 14 tasks contain
acceptance criteria, verification commands, file scopes, and earlier dependencies.
No application tests/build can run before scaffolding.

Independent Gemini 3.8 Flash medium review was attempted but timed out after
240 seconds without a result. Its stderr also warned that combining plan mode
with disabled slash expansion negated plan mode; do not reuse that combination.
No reviewer process remained after timeout and no unexpected project edits were
observed. No independent findings or approval were claimed from that attempt. A later
independent GPT-6 Luna high review completed under the owner's GPT-only policy;
its high-severity clock-skew/sample-age finding was accepted. The contract now
requires dynamically serialized, server-monotonic ageMs on each metric and a
browser-monotonic expiry that duplicate samples cannot extend.

## Authorized execution

The owner approved implementation and explicitly requested Herdr orchestration,
GPT-only developers/reviewers, and worktrees for parallel features. Bootstrap
shared toolchains first, then isolate backend and frontend work. Frontend work
may proceed against the fixed synthetic contract while transport/security is
implemented; integrate only after review and verification. The unavailable
GPT-6.1 Luna worker may be replaced explicitly with available GPT-6 Luna.
No permission to publish, expose public ingress, or install services is implied.
