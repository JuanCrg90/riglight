# llama-swap inventory verification

Implemented: opt-in model aliases and upstream-reported Loaded / Unloaded /
Unknown states. Loaded is not a readiness assertion. CPU/RAM and GPU protocols
remain unchanged; all three sources share one browser EventSource. Logs,
commands/configuration, request activity/content, inference and model controls
are excluded. No dependency was added or upgraded.

## Results

| Check | Result |
| --- | --- |
| Frontend unit tests | 118 passed |
| Frontend lint, types and production build | Passed |
| Frozen install with scripts blocked | Passed |
| Chromium | 17 passed: nine existing CPU/GPU/security/reconnect cases and eight model cases |
| WebKit | Two request-only API cases passed; 15 browser launches blocked by missing native libraries |
| Go tests with race detector | All packages passed |
| Go vet / gofmt | Passed; formatting output empty |
| Native and no-CGO tests/builds | Passed; primary builds used -trimpath |
| Module verification / govulncheck | Passed; no vulnerabilities found |
| pnpm advisory audit | No known vulnerabilities found |
| Independent GPT review | Backend, frontend, browser cases and relevant fixes reviewed |

The primary reran combined checks after integrating reviewed commits. A review
approval is not another test run. Browser tests use synthetic local fixtures,
separate test-owned loopback ports, no server reuse, no external assets and no
trace/video/screenshot capture. A separate temporary synthetic-only desktop
capture was visually reviewed; no actual inventory was captured. WebKit failures
occurred before browser test bodies ran: two dashboard, five GPU and eight model
cases. Required native
libraries are `libevent-2.1-7t64` and `libavif16`; nothing was installed. Actual
Safari on a Mac remains unverified.

Changed areas: `internal/llamaswap`, guarded server snapshot/SSE integration,
CLI flag/worker lifecycle, frontend inventory parsing/freshness/presentation,
synthetic browser tests, and operator/API/security documentation.

## Passive configured-instance validation

The owner explicitly configured loopback access without authentication and
selected inventory/lifecycle only. Public official API source for compatibility
was inspected before querying the configured service. During the primary's
pre-implementation compatibility discovery, bounded GET `/health` and
`/api/version` checks targeted the configured llama-swap service to establish
access and find its public release source. These were one-off research probes,
not runtime collector calls; actual version/payload data was not published.
The implemented collector and subsequent inventory comparison use only
GET `/v1/models`. No `/running`, general event stream, logs, process/command/
configuration or inference/control endpoints were contacted.

A separate test-owned Riglight listener enabled inventory collection. Eight
fresh model frames matched the configured service's projected aliases and
reported states; only comparison booleans were retained. The live mode and
exact outgoing field allowlist were checked, and CPU publication continued.
No aliases, model counts, raw bodies, installed version, URLs, credentials,
commands, logs or request content appear in this report or fixtures. The owner
server was not started/stopped/restarted. All verification-owned processes
were stopped.

## Security, correctness and cleanup coverage

- Strict literal-loopback URL validation, fixed GET `/v1/models`, no DNS,
  credentials, query/fragment, environment proxy, redirects or insecure TLS.
- Whole-request timeout, parent cancellation, body/header caps and UTF-8/JSON
  validation, including unsafe/duplicate IDs and invalid surrogate escapes.
- Only id/state projection; unknown metadata is discarded, no raw diagnostics
  or source URL forwarding. All rows are validated before the 128-row cap.
- Immutable hub copies, empty arrays instead of null, dynamically aged cached
  samples and exact mode/status/reason/age/null combinations.
- Independent six-second inventory freshness with 32 timestamp identities;
  CPU/GPU-only events and replay cannot renew it. Clock rollback remains valid
  for unseen timestamps; mode changes reset identity state.
- Disabled/demo modes never construct the live HTTP source. Fakes exercise
  continued CPU/GPU/HTTP during a blocked source, cancellation, idle-connection
  cleanup and the existing shared total five-second shutdown budget.
- Accessible reported-state labels, pending/disabled/empty/auth/error/unsupported/
  stale/truncated states, plain-text alias escaping, one EventSource, keyboard
  focus, no console errors/external requests, and 320/768/1440 layout checks.

## Commands and publication gate

Commands used with the pinned toolchain include:

```sh
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web test
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mise exec -- go test -race -p 2 ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal web
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
CGO_ENABLED=0 mise exec -- go test -p 2 ./...
CGO_ENABLED=0 mise exec -- go build -trimpath -o bin/riglight-no-cgo ./cmd/riglight
mise exec -- go mod verify
mise exec -- pnpm --dir web exec playwright test --project=chromium --project=chromium-gpu --project=chromium-models --workers=2
RIGLIGHT_BINARY="$PWD/bin/riglight" mise exec -- pnpm --dir web exec playwright test
mise exec -- pnpm --dir web audit --audit-level low
```

The full browser command intentionally reports the WebKit launch failures;
these are not skipped or presented as passes. Pinned govulncheck and Gitleaks
were used separately. Final publication checks include tracked/history/export/
frontend secret scans, personal-path/source-map artifact inspection, unchanged
exact upstream notices and a clean Git state. Private helper/result paths are
not published. Nothing was pushed/published or installed as a service.

Default collection is still disabled. Enable live GPU and inventory explicitly
with `./bin/riglight --gpu --llama-swap`, or use
`./bin/riglight --demo --gpu --llama-swap` for wholly synthetic examples.
Further lifecycle detail requires a safe upstream API, not exposing the excluded
command/request endpoints. Authentication, logs and controls require separately
approved increments.
