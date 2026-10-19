# Spec: selected-container monitoring through a filtering helper

## Approval and objective

The owner approved Docker state/health, restart counts and CPU/RAM, explicitly
selected containers only, using a restricted proxy. The owner then selected a
separate filtering helper rather than inventory-only delivery. Implement/test
both helper and dashboard integration, but do not install/start the helper,
contact the real daemon, grant socket/group permissions, change Docker/services,
or restart the owner's current dashboard. Live deployment is a later decision.
No logs, controls, shell/exec, images, labels, environment, commands, mounts,
network details, process information, persistence or export in the public API.

Existing toolchain, Go standard library, React/TypeScript and shared telemetry
hub suffice; no new dependencies, CI changes or global tool changes are planned.
Public fixtures/examples/reports remain synthetic. Actual references/aliases,
socket paths and settings stay private runtime configuration.

## Trust boundaries and architecture

Docker inspect contains state/health/restart counts alongside sensitive config
and healthcheck output. A GET-only raw proxy still exposes that response. Use a
separate executable, riglight-docker-proxy, that alone has separately provisioned
Unix-socket access. It makes only two fixed GET operations for configured
containers, decodes a small projection, discards unknown fields and returns only
the documented safe DTO. Its socket access remains privileged even though its
code is read-only: do not run as root or add Docker permissions automatically.
The helper necessarily receives bounded inspect responses transiently; it must
never forward/store/log unknown configuration or healthcheck output. No logs,
events, list-all, info, version, archive, attach or mutating Docker endpoints.

```text
configured Unix socket -> filtering helper -> loopback projected snapshot
                                                 |
                             Riglight optional HTTP collector
                                                 |
                          one existing SSE -> selected-container UI
```

The helper owns selection: repeat --container alias=reference (1..16). Aliases
are unique bounded ASCII [A-Za-z0-9][A-Za-z0-9_.-]{0,63}; references are exact
Docker names with that character set (up to 128 characters), or full lowercase
64-hex IDs, never prefixes, paths, URLs, regexes or browser input. Reject duplicate
references. Aliases, not daemon names/IDs, cross the helper boundary. Resolve a
reference with inspect: full-ID references must equal the returned full ID;
name references must match returned Name exactly as '/' + reference (one leading
slash, no other normalization). Validate a lowercase 64-hex returned ID and
require stats.id to match it. Then request stats by that full ID. Recreated
containers reset CPU
baselines. No automatic discovery or unselected-container requests.

## Helper behavior

- CLI: explicit absolute --socket path, repeatable --container and canonical
  loopback --listen (default 127.0.0.1:9123). No TCP daemon, environment discovery,
  API negotiation, root requirement or socket-permission changes. Linux live
  implementation; synthetic tests use test-owned temporary Unix sockets.
- Pin upstream API v1.44. Request only GET /v1.44/containers/{reference}/json
  and GET /v1.44/containers/{validated-id}/stats?stream=false&one-shot=true.
  Neither browser nor helper HTTP request selects upstream paths/containers.
  Unsupported API is explicit, not silently retried through broader endpoints.
- Five-second shared collection cadence, at most four concurrent container
  acquisitions, one-second per-request deadline and four-second total poll
  budget. This is a hard cancellation budget, not a promise to finish all 16 at
  worst-case deadlines: return UNAVAILABLE/null for timed-out or unstarted rows,
  clear their CPU baselines, preserve successes with their own completion times,
  and rotate the starting container each poll so slow rows cannot permanently
  starve later selections. No overlap/backlog. Poll outside HTTP handlers; serve cached snapshots
  so requests do not amplify daemon work. Body cap one MiB per daemon response,
  bounded headers, no redirects/proxies, context cancellation and idle cleanup.
- Capture each inspect/stats completion independently with monotonic clocks.
  No stale value is carried into a failed collection. One container's errors
  must not affect other rows or CPU/GPU/llama-swap in the main application.
- CPU uses deltas of total_usage/system_cpu_usage across successful one-shot
  polls, times online_cpus (fallback bounded percpu array length), times 100.
  First sample, resets, identity/count changes and invalid system delta give
  NOT_READY/UNAVAILABLE, never a fabricated zero. Valid zero CPU delta gives 0.
  CPU can exceed 100: 100% means one core; bound to 4096 CPUs / 409600%.
- Memory: usage minus total_inactive_file (cgroup v1) or inactive_file (v2),
  consult v2 only if the v1 key is absent. If v1 exists but is invalid, do not
  fall back to v2: report UNAVAILABLE. Require valid cache <= usage; equality
  yields genuine zero usage. Missing both fields means UNSUPPORTED rather than
  inventing a working set. Report
  memory_stats.limit as reported limit, not a configured quota guarantee. Bound
  used/total to safe JSON integers, 0 <= used <= total and total > 0.
- CPU/memory only while the validated state is RUNNING; otherwise unavailable
  and clear baseline. Health is coarse reported health, not application readiness.
  Absent Health means NOT_CONFIGURED; unknown enum means UNKNOWN. RestartCount
  is Docker's reported counter, not a count reconstructed from events.
- Exact configured loopback Host; reject any Origin header (machine API only),
  queries/bodies and non-GET methods. Only /healthz and /api/v1/containers.
  Keep no-store, nosniff, no-referrer, frame/CSP headers and bounded HTTP timing.
  Host/Origin guards do not authenticate local processes: trusted local processes
  can read the selected projected aliases/metrics. This matches the workstation's
  single-operator loopback trust model; no local multi-user isolation is claimed.
  /healthz reports process liveness only. No raw errors/diagnostics in responses
  or startup logs. Shutdown has one bounded five-second total budget.

## Projected contract (helper and dashboard)

GET /api/v1/containers and named containers events use the same JSON shape.
Helper publishes LIVE only. Main dashboard synthesizes DISABLED/DEMO itself.
Top-level status: DISABLED, NOT_READY, AVAILABLE, UNAVAILABLE or UNSUPPORTED.
Reason codes: null or DISABLED, CONNECT_FAILED, TIMEOUT, AUTH_REQUIRED,
QUERY_FAILED, INVALID_RESPONSE, API_UNSUPPORTED as appropriate. Match existing
GPU/models patterns. Non-AVAILABLE overviews have ageMs null and containers [].
AVAILABLE has 1..16 unique aliases sorted lexically and nonnegative safe ageMs.
No truncation/discovery: too many configured/projected rows are rejected.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:05Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "ageMs": 0,
  "containers": [{
    "alias": "demo-service",
    "metadata": {
      "status": "AVAILABLE",
      "sampledAt": "2026-01-01T00:00:05Z",
      "ageMs": 0,
      "state": "RUNNING",
      "health": "HEALTHY",
      "restartCount": 0
    },
    "cpu": {
      "status": "AVAILABLE",
      "sampledAt": "2026-01-01T00:00:05Z",
      "ageMs": 0,
      "percent": 125
    },
    "memory": {
      "status": "AVAILABLE",
      "sampledAt": "2026-01-01T00:00:05Z",
      "ageMs": 0,
      "usedBytes": 268435456,
      "totalBytes": 1073741824
    }
  }]
}
```

Metadata status AVAILABLE, NOT_READY, UNAVAILABLE, NOT_FOUND or UNSUPPORTED.
AVAILABLE metadata has a completion timestamp/age, state enum CREATED, RUNNING,
PAUSED, RESTARTING, REMOVING, EXITED, DEAD or UNKNOWN, health STARTING, HEALTHY,
UNHEALTHY, NOT_CONFIGURED or UNKNOWN, and nullable safe nonnegative restartCount
(null if missing/invalid). Other metadata statuses have state, health, restartCount and
age/timestamp null. CPU/memory statuses AVAILABLE, NOT_READY, UNAVAILABLE or
UNSUPPORTED; non-AVAILABLE means timestamp/age and all measurements null.
All lists always arrays. No raw identity, upstream timestamps or sensor payloads.

All sampledAt values are UTC. Helper MarshalAt dynamically adds monotonic elapsed
age to retained completion clocks. Main's HTTP collector preserves each upstream
age/timestamp identity, adds elapsed time including full request duration
conservatively and retains a local monotonic reading: never reset cached upstream
age to zero on successful fetch. Frontend rejects malformed/oversized/unsafe DTOs
and expires overview/metadata/CPU/memory independently after 15 seconds using
server age plus browser-monotonic elapsed time. Existing bounded timestamp replay
protection (32 identities, mode resets, no renewal through failure or A-B-A) must
apply. Wall-clock ordering never determines freshness. Stale overview hides rows;
stale metadata hides its values, and stale sensors hide their measurements.

## Main application integration

- --docker opt-in. Live mode requires explicit --docker-proxy-url; URL flag without
  enablement fails. URL is an HTTP/HTTPS literal canonical loopback origin, no
  DNS, userinfo, query, fragment, non-root path, redirects/environment proxies;
  normal TLS verification. No Docker Unix socket access in the dashboard.
- Disabled/demo never construct live source. --demo --docker is all synthetic,
  including aliases/states, and never creates the helper or queries Docker.
- Five-second independent source worker, two-second proxy request timeout,
  128-KiB response cap and bounded headers; require LIVE mode from the helper
  (the dashboard owns DEMO/DISABLED), strict projection/validation and
  generic failure reasons. Close idle connections within existing shutdown budget.
- Add optional container hub to server, GET /api/v1/containers and named
  containers SSE events on the same existing EventSource/writer/client slot.
  Preserve exact Host/Origin, admission modes, input guards and eight-client limit.
- Accessible compact selected-container view; no links, images, logs, controls,
  history or export. Show aliases as escaped text. Explain disabled/setup,
  pending, missing container, proxy/auth/unsupported/error, stale and per-sensor
  unavailability. Label CPU's one-core basis and memory's cache subtraction and
  reported limit. Existing CPU/GPU/models must remain independent and usable.

## Files, tasks and verification

Use internal/containers for contracts/hub/demo/proxy source;
internal/dockerproxy for filtered daemon collector/HTTP boundary;
cmd/riglight-docker-proxy for helper lifecycle; cmd/riglight/containers.go and
internal/server for main wiring; web/src/containers*, ContainerSection* and
single existing hook; synthetic web/e2e/containers.spec.ts on test-owned 19093.
Follow tasks/docker-plan.md; small tests-first commits, gofmt/strict TS, no new
frameworks. Example implementation style: tiny projected structs with explicit
JSON tags, context-aware HTTP calls and generic errors, never upstream DTO export.

Verify source fixtures and exact outgoing method/path/query allowlist, timeouts,
size limits, projection of adversarial secret/config/log sentinels, selections,
identity/reset math, malformed fields and numeric bounds, proxy request security,
all freshness/replay boundaries, cancellation/race/single SSE and disabled/demo
no-source behavior. Use only synthetic fake daemon/proxy data. Native/no-CGO
Go tests/race/vet/builds, frontend frozen install/test/lint/types/build, Chromium
and WebKit gates (report actual launch blockers), audits/notice checks, full
history/export/artifact privacy scans, independent GPT security and UI reviews.
Do not call an actual daemon or change the running app for verification.

Build after frontend assets: mise exec -- pnpm --dir web build;
mise exec -- go test -race ./...; mise exec -- go vet ./...;
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight;
mise exec -- go build -trimpath -o bin/riglight-docker-proxy ./cmd/riglight-docker-proxy.
Synthetic app: ./bin/riglight --demo --gpu --llama-swap --docker.
Operator documentation may show synthetic helper/proxy invocations; do not deploy
or install them. Live integration remains unverified until separately configured.

## Official evidence

- API v1.44: https://docs.docker.com/reference/api/engine/version/v1.44/ and
  https://github.com/moby/moby/blob/615dfdf67264ed5b08dd5e86657bf0e580731cea/api/swagger.yaml
- Current inspect/state/health/stats source, inspected without assuming the
  installed engine version:
  https://github.com/moby/moby/tree/a5ffd75e61b809183b7a0a29551864c9295c78ff/api/types/container
- Stats one-shot semantics and current CPU formula:
  https://github.com/moby/moby/blob/a5ffd75e61b809183b7a0a29551864c9295c78ff/api/swagger.yaml
- CLI working-set memory calculation (total_inactive_file on v1, inactive_file
  on v2, not the older cache field):
  https://github.com/docker/cli/blob/6f8369014a5eedea2764ac6d95eb29115d9a13dc/cli/command/container/stats_helpers.go
  The Swagger prose still mentions the older v1 cache formula; follow current
  CLI's documented inactive-file semantics, with explicit unavailable handling
  for invalid/missing counters and mathematically valid zero after subtraction.

No code copied from Docker; stdlib implementation follows documented protocols.
Check legal/notice requirements again if importing or copying upstream code.
