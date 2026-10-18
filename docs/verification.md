# Milestone 1 verification

Status: implemented and independently reviewed; the complete browser/platform
verification gate remains partially blocked. Nothing has been published or
pushed, and no system service or public ingress has been configured.

## Delivered scope

- Linux live CPU/RAM, one shared two-second collector, snapshot API and SSE.
- Embedded React dashboard with reconnecting, pending, unavailable and stale
  states; server-monotonic sample age and browser-monotonic freshness.
- Bounded 450-point in-memory charts with missing/stale-data gaps and GiB labels.
- Explicit synthetic demo mode that bypasses live collector construction.
- Loopback-only CLI, exact Host/Origin guards, safe static serving, and an explicit
  local development-origin exception only with `--dev`.
- MIT project license and required notices for the confirmed Linux runtime graph.

GPU, Docker, llama-swap, logs, alerts, persistence, authentication/public access,
and service installation remain outside this milestone.

## Verification performed

Commands are run from the repository root using the pinned mise toolchain.
Build frontend assets before Go tests/builds that embed them.

| Check | Result |
| --- | --- |
| Go 1.27.2, Node 24.22.0, pnpm 12.14.0 | Installed and versions verified |
| Frozen frontend install with scripts blocked | Passed; also verified in a clean temporary project without node_modules |
| Frontend unit tests | 26 passed |
| ESLint, strict TypeScript check, production frontend build | Passed |
| Go race tests, vet, gofmt cleanliness | Passed |
| Go production build with `-trimpath` | Passed |
| Chromium production-demo browser tests | 4 passed against the final integrated binary |
| Real Linux live-mode smoke | CPU and RAM became available; only status/field checks logged, no actual metric values retained |
| Development proxy runtime check | Allowed dev Origin: 200; hostile Origin: 403; incorrect Host: 403; dev Origin without `--dev`: 403 |
| govulncheck v1.9.0 | No vulnerabilities found |
| pnpm audit | No known vulnerabilities found |
| Go module integrity | All modules verified |
| Independent GPT reviews | Accepted findings fixed and reviewed before integration |
| Dependency notice fidelity | BSD/MIT text compared to upstream with whitespace normalized; standalone Apache license compared byte-for-byte |
| Privacy/secret/artifact scans | Passed: Gitleaks history/public-content/frontend scans; checked operator identifiers/paths absent from 56 public files and 4 build artifacts; no source maps |

Primary executable checks:

```sh
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web test -- --run
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal web
mkdir -p bin
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
mise exec -- pnpm --dir web exec playwright test --project=chromium
mise exec -- pnpm --dir web exec playwright test --project=webkit
mise exec -- pnpm --dir web audit --audit-level low
mise exec -- go mod verify
```

The format check requires empty output, not just a zero exit status.
`govulncheck ./...` was executed with a binary built from the pinned v1.9.0 tool;
the equivalent reproducible invocation is:

```sh
mise exec -- go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

## Browser and platform limitations

- WebKit project: two API/security tests passed; two browser tests could not
  launch because the host lacks `libevent-2.1-7t64` and `libavif16`. The command
  returned failure, not a passing or skipped suite. Browser downloads succeeded;
  no privileged dependency installer or system package changes were performed.
- Safari on a Mac has not been tested. Linux Chromium/WebKit coverage is not a
  substitute for actual Safari verification.
- Interactive `playwright-cli` could not launch its requested Chrome channel
  because system Chrome was absent. The pinned project Playwright Chromium was
  used instead, including a visual check of the synthetic demo screenshot.
- Live monitoring is Linux-only. Non-Linux live mode is rejected before the live
  factory is invoked; demo mode remains separate.

## Review corrections and evidence

Developers and separate reviewers used GPT models through `openai-codex` in
Herdr, with isolated feature worktrees. The coordinator integrated verified
commits and performed final runtime checks. Accepted corrections included:

- Clock-independent per-metric age instead of comparing workstation/Mac clocks.
- No freshness renewal or duplicate chart points from cached/replayed samples;
  gaps for samples stale at receipt and bounded histories after 450 samples.
- Correct method/body validation order and unknown-length body rejection.
- Real HTTP SSE lifecycle tests, not only custom ResponseWriter tests.
- A dev proxy that changes Host but preserves caller Origin for validation.
- Canonical loopback authorities, safe JSON integer bounds, and Linux live guard.
- CPU intervals with regressing counters become unavailable and recover from a
  reset baseline. Linux guest time is counted once: the eight non-guest fields
  already include guest time in user/nice, so guest must not be subtracted again.
- Correct GiB display and verbatim third-party license packaging. Final exact
  comparison caught and corrected a one-word BSD notice transcription error.

Reviewer findings were triaged against evidence: pnpm 12's multi-document
package-manager/application lockfile was confirmed by a clean frozen install,
not rewritten on an unsupported reviewer assertion. Go CPU accounting was
checked against the pinned upstream source, not merely writer/reviewer claims.

## Privacy and publication gate

Only synthetic data is used in committed fixtures and browser artifacts.
Runtime traces, screenshots, local settings, dependencies and build outputs are
ignored. Git author attribution intentionally uses the existing identity chosen
by the owner; source-content privacy does not imply anonymous Git authorship.

Gitleaks v8.30.1 found no leaks in Git history, a snapshot of tracked/public
untracked content, or the built frontend. Dependencies, local settings, traces,
and other ignored runtime files were excluded from the public-content snapshot.
A separate check found no checked operator identifiers or home paths in the
public source or executable/frontend artifacts; no source maps were generated.

Scanning does not prove arbitrary content is free of personal information;
review content/history and release artifacts again before publication. Recheck
notices if dependencies or supported build targets change.
