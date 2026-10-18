# Spec: Riglight

Status: Milestone 1 implemented and reviewed; WebKit/Safari verification remains
incomplete. See `verification.md` for checks, results, and limitations.

## Objective

Build a private, read-only dashboard for a single Linux AI workstation,
viewable in current Safari and Chrome on macOS over a VPN or SSH tunnel.
Replace the need to watch separate system, GPU, llama-swap, and Docker terminals
for routine monitoring. Do not implement a remote shell or control plane.

Success means a user can see whether the host is healthy, which models are
active where the installed upstream API exposes that state, and relevant live
logs without exposing the workstation publicly.

## Confirmed choices

- Name and executable: Riglight / `riglight`.
- License: MIT, attributed to Riglight contributors rather than a personal name.
- Backend: Go; frontend: React, TypeScript, Vite.
- Toolchain: a project-local mise configuration with a pinned Go version.
- Verification: backend tests/race/vet/format/build, frontend tests/lint/types/
  build, browser smoke testing, dependency vulnerability checks when available.
- Privacy-safe public source and runtime defaults are project requirements.

## Assumptions and boundaries of the first release

- One host, one trusted operator, read-only operations.
- Linux is the monitored platform; macOS is a browser client, not initially a
  monitored host. No native macOS application is needed.
- The existing VPN provides private reachability. Riglight must not configure
  the VPN, firewall, public ingress, system services, or Docker permissions.
- No database, alerts, long-term storage, multi-host aggregation, or telemetry.
- GPU vendor and installed llama-swap API capabilities remain unconfirmed.
  Do not inspect or save machine-specific configuration to infer them silently.

## Scope and delivery order

### Milestone 1: a working live system dashboard

- CPU aggregate utilization and RAM used/total, updated every two seconds.
- One shared collector per host, independent of how many browsers connect.
- Initial snapshot plus a same-origin SSE feed; refresh and reconnect work.
  Per-metric age is computed from server-monotonic collection time at send time;
  browser-monotonic elapsed time drives freshness without synchronized clocks.
- Connected, reconnecting, stale, and unavailable states. Missing data must never
  be presented as healthy zeroes. Stale means no valid sample for six seconds.
- Short rolling charts bounded to 450 samples (15 minutes at two seconds) in
  each browser; no persistence and no historical replay guarantee.
- Responsive, keyboard-accessible UI with text labels, reduced-motion support,
  and no external asset requests.
- Synthetic collector fixtures and an explicit demo mode that never reads host
  metrics. Demo mode must be visibly labeled and suitable for public screenshots.
- Local production build served by Go; development Vite server is loopback-only.

### Later vertical increments, not part of milestone 1

1. System detail: swap, disk capacity and aggregate network throughput. Optional
   process information needs a separate privacy review; never expose full args
   or process environment by default.
2. GPU: utilization, VRAM, temperature, power, and availability. Prefer NVML for
   NVIDIA; unsupported hardware must produce a clear unsupported state. GPU
   process details require explicit privacy choices before collection.
3. llama-swap: model inventory and lifecycle states supported by the installed
   version; an opt-in upstream log viewer with pause, follow-tail, and filtering.
   Verify official API support before committing to endpoint names or metrics.
   No inference requests, prompts, responses, tokens/sec, or queue estimates
   unless explicitly supported and separately approved.
4. Docker: container state/health, restart counts, CPU/RAM, and opt-in selected
   container logs. No start/stop/restart/exec operations. Container names/images
   are potentially sensitive and must not appear in public fixtures or reports.
5. Private deployment documentation and an example systemd unit; do not install
   or enable a service automatically. Prefer a narrow Docker API proxy over
   granting the application unrestricted daemon access.

Each increment needs its own acceptance tests and verification checkpoint.

## Technical architecture

- Go `net/http` backend serves the built frontend and a versioned JSON/SSE API.
  Go is chosen for straightforward host integrations, concurrency, and deployment.
- React/Vite is a static client. SSR and a separate frontend production server
  are unnecessary for this dashboard.
- SSE carries one-way live updates with native browser reconnection. WebSockets
  are unnecessary until bidirectional features actually exist.
- Prefer `gopsutil` for host metrics. Evaluate `nvml` bindings and the official
  Docker client before their increments. Do not parse terminal UI output.
- No database. Bounded memory only; restart discards collected history.
- Slow subscribers cannot block collection: keep a bounded latest-sample queue,
  replace outdated queued samples, bound stream connections, set per-write
  deadlines, and release resources on cancellation/shutdown.
- Collectors use contexts and report explicit availability and sample timestamps.
  Do not return raw dependency errors, absolute paths, or configuration values.
- Start with at most eight simultaneous SSE connections; document the limit.

### Planned API contract

Finalize response schemas and examples during planning, before implementation.
All public examples must be synthetic.

- `GET /healthz`: process health only; not an assertion all collectors are healthy.
- `GET /api/v1/snapshot`: latest allowlisted sample and collector availability.
- `GET /api/v1/events`: SSE snapshot events with sample timestamp and schema version.
- Reject unsupported methods. No mutable API or browser-supplied upstream URL.
- No replay promise: reconnecting clients receive the latest sample rather than
  unbounded catch-up. Do not put sensitive data into URLs or event IDs.

## Privacy and threat model

### Assets and trust boundaries

Assets: credentials, model prompts/responses, identifying host details, Docker
privileges, sensitive logs, and workstation availability.
Boundaries: browser-to-server, collectors-to-host, later server-to-Docker and
server-to-llama-swap, upstream log content-to-DOM, and local files-to-public Git.

### Required controls

- Default bind: `127.0.0.1:9090`, configurable locally. Avoid llama-swap's commonly
  used port 8080. Do not listen on all interfaces by default.
- No built-in public access or authentication flow in milestone 1. Remote use
  requires an SSH tunnel or an access-restricted private HTTPS proxy. VPN-wide
  reachability alone is not authorization; define proxy identity/access policy
  before any non-loopback exposure. Never auto-trust forwarded headers.
- Enforce allowed Host values and reject disallowed Origin values on API/stream
  requests; no wildcard CORS. Test DNS rebinding and hostile browser origins.
- Same-origin frontend assets; CSP, nosniff, framing protection, no-store for
  monitoring APIs, and strict referrer policy. HSTS only at HTTPS termination.
- No root requirement. No Docker access in milestone 1. A read-only socket mount
  does not restrict Docker API privileges and is not an adequate control.
- Use allowlisted metric fields. Do not collect hostname, usernames, IP/MAC
  addresses, disk mount paths, process arguments/environment, or credentials in
  milestone 1. Do not put them into application logs or errors either.
- No analytics, crash-report uploads, external fonts/CDNs, or runtime network
  calls other than explicitly configured integrations. Dependency installation
  and developer security checks may contact public registries/advisory services.
- Do not contact llama-swap or Docker until their integrations are configured.
  Upstream locations are operator-configured, never browser-selected; validate
  schemes/destinations and disallow unexpected redirects.
- Later log viewing is disabled by default and must warn that logs may contain
  sensitive data. Never persist/export by default. Cap line bytes, total buffer
  bytes, line count, and concurrent streams; define exact limits in that increment.
  Render log/model/container text as plain escaped text, never executable HTML.
- Redaction is defense-in-depth, not a guarantee that arbitrary logs are safe.
- Use only synthetic examples, tests, screenshots, and bug-report attachments.
- Before publishing, inspect files and Git history for personal information and
  run secret scanning. Ignore rules and scanners alone do not ensure privacy.

## Project structure (planned)

```text
cmd/riglight/          CLI entrypoint and lifecycle
internal/metrics/     Allowlisted sample types and collectors; colocated Go tests
internal/server/      API, SSE, static serving, and HTTP security tests
web/src/              React components, styles, API client, colocated UI tests
web/e2e/              Browser smoke and privacy/network tests using synthetic data
docs/                 Specification, architecture decisions, operator docs
tasks/plan.md         Implementation plan after spec approval
tasks/todo.md         Dependency-ordered tasks with acceptance and verification
mise.toml             Pinned shared toolchain configuration
```

## Dependency and toolchain policy

Pin supported toolchain and direct dependency versions during scaffolding after
checking official sources, maintenance/release activity, adoption, security
advisories, and license compatibility. Exact versions are not approved by this
draft. Record decisions and versions in the plan and manifests; commit lockfiles.
Use pnpm for the frontend with a pinned packageManager field. Block dependency
install scripts initially and review/allow only specifically required scripts.
No competing package-manager lockfiles or blanket script approval.

## Commands (proposed, not executable until scaffolded)

Run from the project root unless stated otherwise. Scripts must be implemented
with these meanings; do not report these checks as passed before they exist.

```sh
mise install
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
mise exec -- go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
pnpm --dir web install --frozen-lockfile --ignore-scripts
pnpm --dir web test -- --run
pnpm --dir web lint
pnpm --dir web typecheck
pnpm --dir web build
pnpm --dir web test:e2e
pnpm --dir web audit
mise exec -- go run ./cmd/riglight --demo
mise exec -- go run ./cmd/riglight
```

Proposed exact toolchain/dependency pins and secret-scanning commands are recorded
in `tasks/plan.md`. Frontend build must precede any Go build that embeds its assets. A clean format check requires empty `gofmt -l` output, not merely a
zero exit code. Browser tests must start local synthetic/demo services and must
not record actual workstation data. Missing tools/checks must be reported, not
silently skipped. Define and pin the secret-scanning command during planning.

## Code style

Use idiomatic Go, gofmt, explicit errors, small structs/interfaces, and contexts
at I/O boundaries. Use TypeScript strict mode, functional React components, and
plain escaped rendering. No generic plugin framework or unnecessary state store.
Prefer narrow named contracts over leaking third-party response types.

Example style (illustrative, not an implemented contract):

```go
type MemorySample struct {
    UsedBytes  uint64 `json:"usedBytes"`
    TotalBytes uint64 `json:"totalBytes"`
}

func (s MemorySample) Valid() bool {
    return s.TotalBytes > 0 && s.UsedBytes <= s.TotalBytes
}
```

Unavailable samples carry explicit status instead of a fabricated zero value.

## Testing strategy

- Go: standard testing/httptest; injectable synthetic collectors and clocks for
  deterministic tests. Race checks cover shared samples and subscriber lifetimes.
- Frontend: Vitest and Testing Library for display, unavailable/stale states,
  EventSource reconnection handling, and bounded chart history.
- Browser: Playwright smoke test in Chromium and WebKit against demo mode;
  manually confirm Safari on a Mac when available. WebKit is not full Safari
  verification; lack of a Mac must be disclosed rather than claimed as tested.
- Test abuse cases: disallowed origins/hosts, unsupported methods, slow clients,
  disconnect cleanup, stream connection limits, and upstream text injection.
- Privacy tests assert field allowlists and no third-party frontend requests.
  Demo tests verify no host collector is invoked. Inspect built assets too.
- Cover behavior and failure boundaries; do not substitute a coverage percentage
  for acceptance criteria. Record verification commands, results, and blockers.
- Independent review for substantial code changes; report unavailable reviewer
  tooling rather than treating self-review as an independent review.

## Working boundaries

Always:
- Keep increments small and runnable; test behavior before expanding scope.
- Follow the privacy field allowlist and synthetic-data policy.
- Review status/diff before edits and before any publication.
- Keep implementation evidence and unresolved concerns in the handoff.

Ask first:
- Adding dependencies beyond the reviewed implementation plan or changing CI.
- Adding authentication flows, public ingress, new data categories, persistence,
  mutable actions, privileged access, or new external integrations.
- Installing/enabling services or changing firewall/VPN/Docker configuration.
- Creating a remote repository, branches, or publishing/pushing anything.

Never:
- Commit real credentials, host identifiers, logs, screenshots, or local settings.
- Bypass failed tests/hooks, execute model/log content, expose a remote shell,
  silently grant Docker/admin permissions, or hide missing verification.

## Milestone 1 acceptance criteria

1. From a clean documented setup, a production build serves the dashboard and
   same-origin API; real CPU/RAM samples update about every two seconds.
2. Multiple browser tabs share one collector. Stream cleanup, connection limits,
   slow-client handling, and shutdown pass deterministic tests and race checking.
3. Disconnects and collector failures produce correct reconnecting/stale/error
   states instead of misleading zeroes. History remains within 450 samples.
4. Loopback-only binding and Host/Origin/security-header controls pass tests;
   no privileged host access or third-party browser requests are required.
5. Demo mode produces only synthetic data and supports browser tests/screenshots
   without touching host metrics. API fields and public files follow privacy rules.
6. Full agreed verification is run and reported, with any unavailable hardware,
   browser, advisory service, or review tool explicitly identified as a blocker
   or limitation. No GPU/Docker/llama-swap integration is claimed in milestone 1.

## Review gate and open questions

Milestone 1 was approved, implemented in isolated GPT-only Herdr worktrees,
reviewed, and integrated. The initial Git checkpoint preserves the original
draft. See `tasks/todo.md` and `docs/verification.md` for implementation evidence
and the remaining full-browser verification gate.

- Complete WebKit native-library/Safari verification before claiming full Mac
  browser compatibility.
- Confirm GPU vendor and llama-swap version when starting those integrations;
  these are not prerequisites for the CPU/RAM milestone.
- Private HTTPS proxy/access policy and log-viewer limits are decisions for their
  own increments, not implicit permissions granted by this specification.
