# Spec: optional system detail

## Approved scope

Owner requested system detail: swap usage, disk capacity and aggregate network
throughput. They explicitly selected the root filesystem only and explicitly
configured uplinks, not all-interface totals. Implement/test this optional section
without restarting/reconfiguring the running dashboard/helper, selecting actual
interfaces, installing services, changing permissions or querying live host details
for verification. Live interface selection/enablement is a separate later step.
No processes, addresses, MACs, interface/device names, mount paths, filesystem
labels/types, inodes, disk I/O, logs, controls, history or export in public DTOs.

Use pinned Go/gopsutil v4.26.9 and existing React/TypeScript; no new dependency or
framework is planned. Verify the actual linked dependency/license graph anyway.
Public fixtures/examples/screenshots/evidence are synthetic only.

## CLI and acquisition

- --system-details explicitly enables collection. Default disabled. Repeatable
  --network-interface names select 1..8 unique exact interfaces in LIVE mode;
  missing live selection fails closed. Reject selection flags without enablement,
  duplicate/empty/lo names, paths, patterns, whitespace, NUL/control/Unicode names,
  overlong names or too many. Accept [A-Za-z0-9][A-Za-z0-9_.:-]{0,14}; sort selection
  internally. These are private runtime configuration, never API output.
- --demo --system-details is all synthetic and needs no interface flags. If
  selection flags are supplied in demo, validate them but do not construct/read
  any live source. Preserve Linux-only live platform policy and non-Linux demo.
- Independent two-second shared worker; one acquisition per tick, no overlap,
  per-client collection or new network calls. Worker cannot stall existing CPU,
  GPU, models, containers or HTTP. Reuse the existing total five-second shutdown
  deadline. No unbounded spawned goroutines to abandon blocked syscalls.
- Use projected injected numeric providers for tests. The Linux adapter uses
  standard syscall.Sysinfo for raw swap units and syscall.Statfs("/") for raw
  block counts, then checked arithmetic described below (gopsutil's precomputed
  byte arithmetic is not trusted to detect overflow). Existing pinned
  net.IOCountersWithContext(ctx, true) supplies private per-interface RX/TX counters
  only for exact configured selections. Native adapters are Linux build-tagged;
  unsupported-platform stubs keep non-Linux demo/builds working. Do not call
  partition/interface/address/process discovery or serialize kernel/library
  structs. Unknown fields and raw dependency errors never cross the boundary.
- Metric failures are independent: swap failure does not fabricate disk/network
  failure or stale carry-forward. Capture each successful sensor's completion
  timestamp and monotonic clock after its call. Root statfs and the proc readers
  used here do not enforce cancellation of underlying syscalls:
  a blocked system worker may make all system fields stale, but cannot block
  other sources/HTTP or extend the shared shutdown budget. Do not claim a hard
  kernel-call timeout or unload/interrupt libraries unsafely.

## Measurement semantics

Swap: validate raw unit bytes > 0 and freeUnits <= totalUnits BEFORE subtraction;
check totalUnits*unitBytes and (totalUnits-freeUnits)*unitBytes for overflow and
safe JSON bounds before producing bytes. Invalid input means UNAVAILABLE.
Safe integer bytes satisfy 0 <= used <= total. total=0 with used=0 is an AVAILABLE
fresh observation of no configured swap, not unsupported, failed or a fake zero.
If total>0 and used=0, this is genuinely idle swap. Do not expose paging counters.

Root filesystem: used=(Blocks-Bfree)*Bsize, available=Bavail*Bsize, total from
statfs. Validate block size > 0, Bfree <= Blocks and Bavail <= Bfree BEFORE
subtraction. Check each multiplication for uint64 overflow and safe JSON bounds;
invalid raw values mean UNAVAILABLE, never wrapped plausible bytes. Bavail is
user-available capacity, not all unallocated blocks (as in gopsutil's Free).
Require safe integers, total>0, used/available>=0 and used+available<=total.
Reserved/unavailable bytes may account for the difference: never force used+
available=total. If showing percent, calculate used/total, not the library's
UsedPercent (which uses a different denominator). Always label Root filesystem,
Used, Available and Total; no paths/types/devices/mount discovery.

Network: sum interval RX/TX byte deltas across configured uplinks, divided by
actual monotonic seconds between successful counter-completion readings. Keep
individual selected-interface baselines so a reset cannot be hidden by growth on
another interface. Validate all selected names exactly once before calculating;
missing/duplicate selected rows or provider failure produce UNAVAILABLE and clear
baseline. Ignore unselected data. First sample, any per-interface decrease and
nonpositive elapsed time produce NOT_READY and re-baseline. Valid unchanged
counters produce 0 B/s. Use checked uint64 delta aggregation and finite nonnegative
rates <= Number.MAX_SAFE_INTEGER B/s; overflow/invalid result means UNAVAILABLE
and clears baseline. Raw cumulative counters and interface names stay internal.
Traffic includes whatever the operator selected; do not infer internet speed,
connectivity, link health or automatically exclude additional selected adapters.

## Frozen JSON contract

GET /api/v1/system and named system events on the existing single SSE carry:

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "swap": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":1073741824,"totalBytes":4294967296},
  "disk": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":214748364800,"availableBytes":268435456000,"totalBytes":536870912000},
  "network": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"rxBytesPerSecond":1048576,"txBytesPerSecond":262144}
}
```

Mode DISABLED, DEMO or LIVE. Sensor statuses DISABLED, NOT_READY, AVAILABLE,
UNAVAILABLE, UNSUPPORTED. DISABLED mode requires all three sensors DISABLED;
enabled modes cannot contain DISABLED sensors. Only AVAILABLE has UTC timestamp,
nonnegative safe integer ageMs and nonnull allowlisted values; every other status
has timestamp/age/all measurements null. UNSUPPORTED is allowed in LIVE/DEMO
when an injected provider explicitly reports absent sensor capability through a
project sentinel (or the Linux adapter maps ENOSYS/EOPNOTSUPP); ordinary errors,
missing interfaces and malformed readings are UNAVAILABLE. No swap configured is
AVAILABLE, not UNSUPPORTED. Demo fixtures may simulate capability absence; no raw
error-string matching. Exact top-level and sensor key sets are those shown above:
all keys are required, no unknown keys, no omitted null fields. Go serialization
uses only the typed allowlist and validates all values; browser parsers reject
extra/missing keys. Top sampledAt is attempt time, not
proof a sensor succeeded. Do not add top-level overview flags or identity fields.
AVAILABLE byte counts are safe integers; RX/TX rates can be fractional finite
numbers. Arrays/parameterized disk inventories are deliberately not included.

MarshalAt retains per-sensor monotonic completion clocks and computes age when
serializing, not at publication. Browser freshness is age plus monotonic elapsed,
expires independently at six seconds and ignores wall-clock ordering. Maintain
32 timestamp-expiry identities PER sensor, persist shortened expiries, preserve
identities through failure, reset on mode change and let timer logic only expire
existing deadlines. Use LRU retention of the most recently encountered 32
identities per sensor; guarantees apply while an identity is retained. Same
retained timestamps/A-B-A/reconnect/other-source frames never renew freshness.
An evicted identity may be treated as unseen: there is no unbounded anti-replay
claim. Server-computed age still makes genuinely old current snapshots stale.
Unseen timestamps after wall-clock rollback remain valid.

## Server/frontend behavior

Optional system hub and /api/v1/system use all existing method/query/body,
Host/Origin, loopback/tailnet/source admission and security/no-store controls.
Add named system frames as fifth source on the existing writer/EventSource and
eight-client limit; atomic subscription/current snapshot and cancellation remain.
Default process publishes a valid disabled snapshot, demo enabled is synthetic.

Accessible compact System detail region: Swap, Root filesystem and Selected
uplinks receive/transmit. Show GiB and byte-rate units with clear RX/TX labels,
valid zero/no-swap states, disabled/setup/pending/unsupported/unavailable/stale
states. Hide expired values rather than displaying healthy old numbers. No extra
EventSource, initial fetch, charts, identities, links, controls or persistent data.
Preserve all other source behavior, mode binding and StrictMode cleanup.

## Ownership, tasks and verification

Go writer: internal/systemmetrics/**, cmd/riglight/system.go and minimal required
main/server source/tests. Frontend writer: web/src/system*, SystemSection* and
minimal existing hook/App/style/tests. Primary owns docs/tasks/browser cases and
integration. Use isolated GPT-only Herdr worktrees, one writer per file, no worker
delegation/architecture changes or actual host/interface/daemon investigation.

Tests first: contract/nulls/modes/zero-swap/reserved-disk bytes, strict numeric
bounds/identity omission, providers fail independently/nil structs, selected-set
missing/duplicate/reset/overflow and monotonic nonuniform rate intervals, genuine
zero counters, first/recovery baseline, single worker/cancel/blocking/shutdown,
CLI/factory no-source matrix and all HTTP/SSE guards. Frontend validator/freshness/
mode/reset/replay tests and synthetic Chromium/WebKit project on test-owned 19094
cover fifth-source/one transport, zero/partial/stale states and responsive UI.

Build frontend first. Pinned frontend frozen install/test/lint/types/build, Go
native/no-CGO tests/race/vet/gofmt/build, module/advisory/audits, linked dependency/
notices, privacy/history/export/artifact checks and separate GPT reviews. Disclose
unchanged WebKit native-library/Safari limits rather than skipping failures. No
live accuracy/network/interface selection or app restart is implied by tests.

Commands: mise exec -- pnpm --dir web build; mise exec -- go test -race ./...;
mise exec -- go vet ./...; mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight.
Synthetic app: ./bin/riglight --demo --gpu --llama-swap --docker --system-details.
Live example (synthetic interface): ./bin/riglight --system-details --network-interface example0.

## Official evidence

Inspected exact pinned source and signatures, not memory or terminal output:

- https://github.com/shirou/gopsutil/blob/v4.26.9/mem/mem_linux.go
- https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_unix.go
- https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_linux.go
- https://pkg.go.dev/github.com/shirou/gopsutil/v4@v4.26.9/mem
- https://pkg.go.dev/github.com/shirou/gopsutil/v4@v4.26.9/disk
- https://pkg.go.dev/github.com/shirou/gopsutil/v4@v4.26.9/net

- https://man7.org/linux/man-pages/man2/sysinfo.2.html
- https://man7.org/linux/man-pages/man2/statfs.2.html
- https://pkg.go.dev/syscall#Sysinfo and https://pkg.go.dev/syscall#Statfs

Pinned source is already available in the local module cache. No upstream code
is copied and no new dependency/notices are assumed without linked-graph checks.
