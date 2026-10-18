# Spec: optional llama-swap inventory

Status: implemented and reviewed; see [verification](verification-models.md).
The owner approved this increment after accepting GPU monitoring. They chose
inventory/lifecycle only, loopback access and no authentication. Logs remain out
of scope. Actual instance details and probe responses stay private/ignored.

## Objective and boundaries

Add model aliases and accurately supported coarse lifecycle states to the same
private dashboard. Keep CPU/RAM/GPU behavior, public wire schemas, single browser
EventSource, loopback binding and security guards unchanged. No inference,
load/unload, cancellation, process inspection, configuration reads, prompts,
responses, request activity, tokens/sec, logs or persistence. No new dependency.

Enable explicitly with `--llama-swap`; default is disabled with no upstream work.
Default source is the conventional `http://127.0.0.1:8080`.
`--llama-swap-url` overrides it only when --llama-swap is present; reject a URL
flag without enablement. `--demo --llama-swap` generates synthetic models and
must not construct/use the HTTP source. It works with --gpu and no-CGO builds.
Do not start/restart the owner's server, change services, or generate workloads.
Use only test-owned separate loopback listeners.

## Official API evidence and privacy choice

Official reference: mostlygeek/llama-swap public release v265 and source commit
`a2796fb0306215c6883454a8707dfb22d25c3068`. Public source for the configured
instance's release was checked separately without publishing that installation's
version/configuration. After the owner configured the service, the primary's
pre-implementation discovery used bounded GET /health and /api/version probes
solely to establish access and locate matching public release source. These
one-off research probes are not runtime collector permissions. A private
bounded GET /v1/models probe confirmed inventory/coarse status shape; no aliases,
counts, metadata or actual readings were kept in public files. No need to
import upstream's Go module or copy its code.

- https://github.com/mostlygeek/llama-swap/blob/a2796fb0306215c6883454a8707dfb22d25c3068/internal/server/api.go
- https://github.com/mostlygeek/llama-swap/blob/a2796fb0306215c6883454a8707dfb22d25c3068/internal/server/apigroup.go
- https://github.com/mostlygeek/llama-swap/blob/a2796fb0306215c6883454a8707dfb22d25c3068/internal/server/server.go
- https://pkg.go.dev/net/http#Client
- https://pkg.go.dev/net/http#Transport

The runtime integration must use only `GET /v1/models`, projecting `data[].id` and `data[].status.value`.
Ignore/discard other upstream fields, never forward raw responses or diagnostics.
The source includes optional metadata; it must not enter application DTOs/logs.
Do NOT call `/running`: its objects include cmd/proxy metadata. Do NOT subscribe
to `/api/events`: it includes in-flight requests/UI configuration/activity. Do
not call `/logs`, `/upstream`, `/props`, profiles, inference or control routes.

Current status.value is `loaded` or `unloaded`. Loaded includes non-stopped
processes and does not prove readiness; do not relabel it Running/Ready. Missing
or unrecognized status maps to UNKNOWN, not Unloaded. Empty inventory is valid.
Finer starting/ready/stopping states are deferred until a safe narrow API exists.

## Stable Riglight API (all examples synthetic)

New guarded `GET /api/v1/models` and named `models` SSE event on the existing
stream carry the same schema. Production CLI always supplies an explicit model
hub; nil optional server config may omit this endpoint/event for old callers.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "ageMs": 0,
  "isTruncated": false,
  "models": [
    {"id":"example-chat","state":"LOADED"},
    {"id":"example-embed","state":"UNLOADED"}
  ]
}
```

Exact top-level keys above; models contain only id/state.
- Modes: DISABLED only with overview DISABLED; otherwise DEMO or LIVE.
- Overview statuses: DISABLED, NOT_READY, AVAILABLE, UNAVAILABLE, UNSUPPORTED.
- Reasons: DISABLED requires DISABLED; NOT_READY/AVAILABLE require null;
  UNSUPPORTED requires API_UNSUPPORTED; UNAVAILABLE requires CONNECT_FAILED,
  TIMEOUT, AUTH_REQUIRED, QUERY_FAILED or INVALID_RESPONSE.
- AVAILABLE has ageMs as a nonnegative safe integer and 0–128 models. Preserve
  monotonic completion time internally and calculate age at each serialization.
  All other states have ageMs=null, models=[], isTruncated=false. sampledAt is
  always a valid UTC RFC3339 timestamp for the snapshot/status update.
- States: LOADED, UNLOADED, UNKNOWN. Match upstream values literally; no guesses.
- IDs: nonempty trimmed valid Unicode, at most 128 code points and 512 UTF-8
  bytes; no control/format characters, backslashes, absolute/tilde/drive paths,
  URL schemes or parent-directory segments. Do not trim/rename unsafe IDs into
  apparently valid ones: invalidate the response instead. Org/model aliases
  are allowed. IDs are plain escaped text, never links or HTML.
- IDs are distinct; backend sorts by raw string for stable presentation. The
  frontend validates distinctness, not locale-dependent ordering.
- Cap rows at 128, isTruncated=true only for overflow, with exactly 128 returned.
  Validate all entries in the bounded response before truncating, so unsafe or
  duplicate entries cannot hide beyond the cap.
- Validate the Riglight payload strictly (exact fields/enums/nulls/ranges/UTC),
  unlike the deliberately projected extensible upstream payload.

## HTTP source and lifecycle

`internal/llamaswap` owns sample/hub/demo and HTTP source. Reuse the canonical
internal/telemetry hub, deep copying list/time/reason ownership boundaries.
Collection runs independently every two seconds, never on CPU publication or
request paths, never per browser. Use context cancellation and a two-second
whole-request timeout; cached snapshots age honestly if collection stops.

Source URLs are operator CLI configuration, never browser input. Accept only
http/https with a literal loopback IP and explicit valid port; canonicalize the
address. Reject DNS names, wildcard/non-loopback addresses, userinfo, opaque
URLs, zone IDs, query/fragment, and non-root base paths. Resolve the one fixed
`/v1/models` path. Disable environment proxies and all redirects, keep default
TLS verification (no insecure option), bound response headers and body (1 MiB),
require valid UTF-8 JSON and one complete envelope with a data array. No retries
within a poll; do not follow URLs found in upstream content.

Classify refused/unreachable connections, request deadlines, 401/403, 404/405/501,
other HTTP failures and malformed/oversized responses into the fixed reasons
above. Do not expose underlying errors, URLs, status text or upstream bodies.
No API key flow in this increment; AUTH_REQUIRED is explicit, not auto-remedied.
Source timestamp/monotonic time is captured after acquisition and validation.

Cancel the source on application shutdown and share the existing total five-
second HTTP/GPU cleanup budget. Do not change safe GPU ownership/unload behavior.
CLI tests inject factories to prove disabled/demo never construct the source,
blocked/failing source leaves CPU/GPU/HTTP responsive, HTTP is canceled, and all
subscriptions/worker goroutines clean up. Unit/browser tests use local fakes,
never the operator service. Only the primary performs explicit passive live
validation, retaining booleans/aggregate comparison results, not aliases/counts.

## Browser integration

Keep one EventSource with snapshot, gpus and models named listeners. Independent
inventory age plus performance.now expiry: six seconds means stale; CPU/GPU-only
frames cannot refresh models. Bounded 32 timestamp-expiry identities prevent
cached A→B→A or unavailable→A replay from renewing freshness; do not order wall
clock timestamps (legitimate rollback must work). Mode changes clear state.

Add a compact accessible llama-swap section with plain aliases and Loaded /
Unloaded / Unknown. State clearly that Loaded does not imply ready. Honest
pending/disabled/empty/unavailable/unsupported/auth-required/stale/truncated
states; no controls, links, logs, history or export. Unknown means unsupported
state, not a healthy unloaded model. Keep 320/768/1440 layouts and current theme.
Small focused parser/freshness/presentation helpers; do not build a plugin system
or renovate unrelated CPU/GPU code. Update introductory copy to generic private
workstation telemetry so it stays truthful with integrations disabled/enabled.

## Verification and release boundaries

Follow pinned existing frontend tests/lint/typecheck/build, Go race/vet/gofmt,
native/no-CGO tests/builds, Chromium and attempted WebKit, module/audit/license/
secret/export/artifact checks, plus independent GPT review. Use synthetic local
HTTP fixtures for URL/redirect/proxy/timeout/body/header/status/unknown-fields/
unsafe-ID/truncation tests and all DTO/freshness/stream/lifecycle/UI cases.
Production browser demo uses a third test-owned loopback port if necessary;
no 8080/9090 access in tests. Build frontend before embedding it in Go.

No new runtime/build dependencies, authentication flow, real logs/fixtures,
Docker changes, system installation, publishing/pushing, or scope expansion.
Do not claim blocked WebKit/Safari checks passed. Preserve existing upstream
legal notices and unrelated work. Keep small tested commits and a clean handoff.
