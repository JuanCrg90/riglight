# Spec: optional read-only GPU monitoring

Status: GPU scope approved by the owner after live CPU/RAM accuracy verification.
The primary session owns the technical design and GPT-only Herdr coordination.
This document defines the next bounded increment; no Docker or llama-swap work.

## Objective

See GPU utilization, VRAM used/total, temperature and power alongside existing
CPU/RAM in the private dashboard. Support physical NVIDIA GPUs first, report
unsupported/unavailable readings honestly, and never compromise CPU collection.
No GPU control, process inspection, hardware identifiers or persistent history.

## Scope and safe defaults

- `--gpu` explicitly enables native GPU collection; default is disabled.
- `--demo --gpu` supplies synthetic GPU data without constructing/initializing
  any live GPU/NVML source. Demo without the flag keeps GPU collection disabled.
- Existing CPU/RAM snapshot schema and `snapshot` SSE events are unchanged.
- GPU data uses a new `/api/v1/gpus` endpoint and named `gpus` SSE events on the
  existing stream. The dashboard still opens only one EventSource.
- Expose only local numeric index plus allowed numeric measurements/statuses.
  Do not query or expose GPU names, UUIDs, serials, PCI IDs/addresses, processes,
  command lines, driver versions, application/model names or environment data.
- Collect at most 16 physical devices and report `isTruncated` if the inventory
  exceeds that limit. No MIG instance enumeration or process-level monitoring.
- GPU cards display current values and individual freshness; GPU history/charts
  are deferred. Existing CPU/RAM histories retain the 450-point bound.
- Keep the owner's running server untouched. Verify new builds using separate
  test-owned loopback listeners and explicitly enabled read-only GPU queries.

## Stack, dependency and official sources

Use the current Go/React/Vite versions and add exactly
`github.com/NVIDIA/go-nvml v0.13.4-1` (Apache-2.0) plus required transitives.
Initial health check: official NVIDIA repository, non-archived, recent release
2026-10-06, active commits, and established adoption. Its declared Go version is
1.20, compatible with the pinned Go 1.27.2. Review actual resolved imports,
advisories and all applicable copyright/license notices before handoff.

Read the pinned upstream README, binding source and NVML header for signatures
and semantics. The driver library is dynamically loaded at initialization; a
working driver is not needed just to compile the bindings. Do not regenerate
bindings, run upstream hardware/process tests, install a driver, or change host
configuration. The existing local NVIDIA read-only query was checked without
recording hardware identities or readings.

- https://github.com/NVIDIA/go-nvml/tree/v0.13.4-1
- https://github.com/NVIDIA/go-nvml/blob/v0.13.4-1/pkg/nvml/device.go
- https://github.com/NVIDIA/go-nvml/blob/v0.13.4-1/pkg/nvml/init.go
- https://github.com/NVIDIA/go-nvml/blob/v0.13.4-1/pkg/nvml/nvml.h
- https://pkg.go.dev/github.com/NVIDIA/go-nvml/pkg/nvml

An attempted deep link to the online device-query reference returned 404;
the pinned official header/binding source was used instead. Header notes:
- Utilization is percent; physical-device utilization may be unsupported in MIG.
- Memory is bytes. Prefer the versioned v2 memory query, with a documented v1
  fallback on an older driver. Preserve returned used/total semantics; v2
  includes driver-reserved memory in used memory.
- Temperature is degrees Celsius. Prefer `DeviceGetTemperatureV(...).V1()`;
  the legacy temperature call is deprecated in NVML 13 and is permitted only
  as an explicit compatibility fallback when the modern call is unsupported.
- Power usage is milliwatts: divide by 1000 for watts. On most Ampere/newer
  devices it is a one-second average, not necessarily instantaneous power.

## GPU wire contract

Example below is synthetic. `/api/v1/gpus` and `event: gpus` carry the same JSON.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "isTruncated": false,
  "devices": [{
    "index": 0,
    "utilization": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"percent":42},
    "memory": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":8589934592,"totalBytes":25769803776},
    "temperature": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"celsius":55},
    "power": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"watts":125.5}
  }]
}
```

- Overview status: DISABLED, NOT_READY, AVAILABLE, UNAVAILABLE, UNSUPPORTED.
  AVAILABLE means the inventory was obtained, not that every sensor is healthy.
- reasonCode is null or a fixed generic code: DISABLED, UNSUPPORTED_PLATFORM,
  BACKEND_UNAVAILABLE, DRIVER_UNAVAILABLE, NO_DEVICES, QUERY_FAILED, DEVICE_LIMIT.
  AVAILABLE has null reason when not truncated, and DEVICE_LIMIT when truncated.
  DISABLED requires DISABLED; NOT_READY requires null; UNAVAILABLE requires
  QUERY_FAILED. UNSUPPORTED requires UNSUPPORTED_PLATFORM, BACKEND_UNAVAILABLE,
  DRIVER_UNAVAILABLE or NO_DEVICES. Do not emit raw NVML errors/strings, paths
  or configuration.
- DISABLED/NOT_READY/UNAVAILABLE/UNSUPPORTED carry an empty devices array and
  isTruncated=false. Missing handles in an otherwise available inventory yield
  an indexed device with individual unavailable metrics, not fabricated zeroes.
- Each metric status: AVAILABLE, NOT_READY, UNAVAILABLE, UNSUPPORTED. For anything
  non-AVAILABLE, sampledAt, ageMs and all numeric fields are null.
- AVAILABLE measurements require UTC RFC3339 sampledAt and nonnegative safe
  integer ageMs, dynamically computed at serialization from retained monotonic
  collection time. Browser wall clocks must never drive freshness.
- Percent finite 0–100; memory safe integer bytes, total>0, 0<=used<=total;
  Celsius finite -273.15–200; watts finite 0–10000. Invalid successful driver
  output becomes unavailable, not a misleading accepted number.
- Device indices are distinct integers in 0–15, sorted ascending; devices<=16.
  Strict frontend validation rejects unknown fields, invalid enum/value/null
  combinations, duplicate indices, malformed timestamps or unsafe numbers.
- A cached reconnect cannot renew per-metric freshness. Repeat timestamp expiry
  may shorten, never extend. Six seconds without new valid sensor data means
  stale, even while CPU frames continue arriving normally.

## Backend design and lifecycle

- New `internal/gpu` package owns GPU types, fake/demo source and native adapter.
  Native imports are isolated behind `linux && cgo` build constraints. A no-CGO
  or unsupported-platform build still supports CPU/demo and reports an explicit
  unsupported GPU backend instead of failing the whole executable.
- Extract the current latest-value fan-out into a small typed generic helper in
  `internal/telemetry`, preserving the existing metrics.Hub/Subscription API and
  tests through narrow aliases/wrappers. CPU/GPU reuse its canonical bounded
  queue, clone/validate and cancellation behavior; do not duplicate a second
  concurrency hub or build a generic plugin framework. A receive-only Updates
  channel may expose already-owned queued clones for the SSE select loop;
  retain Next(ctx), idempotent unsubscribe and legacy CPU semantics.
- Separate GPU collection worker: two-second cadence, one owner of Init/read/
  Shutdown, no per-browser driver work. Initialization happens in that worker,
  never on the CPU publication path. Disabled/demo never initialize NVML.
- Native driver calls are not safely cancellable by Go contexts. A blocked GPU
  call must not stall CPU/HTTP collection. Cancel streaming first on shutdown;
  wait for normal GPU cleanup within the same five-second total shutdown budget
  as HTTP, not an additional unbounded/serial wait. Never unload NVML concurrently
  with an outstanding driver call. Document this limit.
- Use a narrow injectable driver facade for deterministic tests; do not call
  live GPU APIs in unit/browser tests. Live hardware checks are explicit only.
- GPU snapshots are immutable after publication; preserve monotonic timestamps
  through copies and calculate wire age per response/frame.
- Existing SSE slot limit remains eight per dashboard stream, not per sensor.
  One writer serializes CPU/GPU frames, each with the existing five-second write
  and flush deadline. Bound per-source queues so GPU cannot starve CPU updates.
- Existing exact Host/Origin/method/body/query guards and headers apply to the
  new endpoint. CPU snapshot keys/status semantics must not change. Legacy
  EventSource clients listening for snapshot ignore the new named GPU event.
- Nil GPU configuration in library-level server tests may omit GPU SSE events;
  the production CLI always supplies an explicit disabled/demo/live GPU hub.

## Frontend design

Keep the current compact palette and accessible CPU/RAM layout. Add a separate
GPU section with generic labels GPU 0, GPU 1, etc. Use prominent utilization and
VRAM (GiB), secondary Celsius/watts, and plain text disabled/unsupported/error
and stale states. Never present unavailable values as zero or update their
freshness from CPU-only events. Do not add another EventSource, external assets,
process controls, persistent storage or additional JavaScript dependencies.

Split small GPU parsing/freshness/presentation helpers from the main hook/App;
keep numeric runtime validation at the boundary and code easy to review. Update
EventSource test doubles to model named listeners correctly, not overwrite the
CPU handler when adding a GPU listener. Use synthetic data in all tests.

## Commands and testing

Existing pinned commands remain authoritative; build web assets before Go embed.

```sh
mise exec -- pnpm --dir web test
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal web
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
CGO_ENABLED=0 mise exec -- go test ./...
CGO_ENABLED=0 mise exec -- go build -trimpath -o bin/riglight-no-cgo ./cmd/riglight
mise exec -- pnpm --dir web exec playwright test --project=chromium
mise exec -- go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
mise exec -- pnpm --dir web audit --audit-level low
```

Go style: gofmt, contexts at I/O/lifecycle boundaries, explicit errors and typed
contracts; no raw library structs in responses. Example:

```go
type OverviewStatus string

const Disabled OverviewStatus = "DISABLED"
```

Test serialization allowlists/null cases/ranges, age and copy isolation, sensor
partial failures, no driver/no-CGO, demo/default isolation, truncation, disposal,
blocked GPU versus healthy CPU, combined SSE limits/slow consumers/disconnects,
legacy CPU schema, per-sensor replay/staleness, and GPU-enabled demo browsers.
Re-run relevant checks after accepted review fixes. WebKit remains blocked by
previously identified missing native libraries; do not install system packages.

## Boundaries and acceptance

Always use synthetic fixtures, exact dependency pins, raw upstream license text,
GPT-only independent review, and isolated feature worktrees. Do not touch the
owner's running process or export actual hardware readings/identifiers in logs,
fixtures, screenshots, public docs or Git. No driver upgrades, system changes,
GPU workload injection, privileged access, mutable API, model/container control,
or expanded integrations. Ask before expanding this agreed scope.

Done means existing CPU/RAM still passes independently, GPU disabled/demo/live
and failure cases are honest, native values are checked against a read-only
numeric reference without publishing them, all applicable checks pass, missing
browser/native capabilities are disclosed, no secrets/private host data enter
the public repository, and all required upstream legal text is preserved.
