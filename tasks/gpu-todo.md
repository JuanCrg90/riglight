# Tasks: GPU increment

Status: authorized scope, implementation pending. Follow `gpu-plan.md` and
`../docs/spec-gpu.md`. Each numbered task is split into small verified commits;
if more than about five files need changing, split the increment further.

## Backend lane

### B1: canonical bounded fan-out
- [ ] Extract the current hub into `internal/telemetry`, preserving existing
  metrics.Hub/Subscription behavior through narrow aliases/wrappers.
- Acceptance: CPU contract, copy isolation, atomic subscribe, bounded queue,
  cancellation and closed-hub tests still pass. No GPU dependency yet.
- Verify: Go race tests, vet, package build, gofmt/diff checks.
- Files: internal/telemetry/hub.go and tests; internal/metrics/hub.go and tests.
- Dependencies: none.

### B2: GPU types and demo
- [ ] Add GPU sample/metric/status validation, per-send monotonic age, immutable
  hub reuse, and deterministic synthetic source.
- Acceptance: exact allowlist/null/range/index/cap semantics, partial statuses,
  no native calls for disabled/demo, and dynamically aging cached snapshots.
- Verify: focused GPU race tests plus existing Go suite, vet/format/build.
- Files: internal/gpu/sample.go and tests, hub.go, demo.go and tests; split as needed.
- Dependencies: B1.

### B3: native optional provider
- [ ] Add pinned NVML dependency, a narrow injectable driver facade, single-owner
  native lifecycle, versioned read APIs/fallbacks, and no-CGO/platform stub.
- Acceptance: numeric sensors/units correct, failures local to sensor, no names/
  IDs/process queries, cap16, no fatal default/driver-absent behavior.
- Verify: synthetic return/unit-conversion/range/lifecycle tests; race/vet/native
  build; no-CGO test/build. Do not access real GPU in unit tests.
- Files: go.mod/go.sum, native adapter/stub/provider and tests; split by concern.
- Dependencies: B2. Authorized project module downloads only, no system installs.

### B4: secure GPU snapshot and shared SSE
- [ ] Add GPU hub/config, guarded GET gpus endpoint and multiplexed GPU events on
  the existing SSE response writer with bounded queues/deadlines and cleanup.
- Acceptance: exact CPU JSON/legacy event behavior preserved, GPU statuses and
  age accurate, one connection/slot, no starvation or leaks on close/error.
- Verify: real HTTP stream tests, hostile inputs/methods, limits, race/vet/build.
- Files: internal/server/server.go, events.go, GPU handlers and focused tests;
  keep behavior-preserving and feature commits separate.
- Dependencies: B2 (can be built against synthetic sources before B3).

### B5: CLI opt-in worker isolation
- [ ] Add --gpu, explicit disabled/demo/live wiring, independent two-second GPU
  loop and safe bounded shutdown without unloading active native calls.
- Acceptance: default/demo never construct live NVML; blocked fake GPU cannot
  stall CPU/HTTP; driver absent/no-CGO is honest unsupported; listener unchanged.
- Verify: injected fake CLI/lifecycle tests, full Go race/vet/format/build,
  no-CGO test/build; no user-owned process or existing port touched.
- Files: cmd/riglight/main.go, GPU lifecycle helper and focused tests (<=5/task).
- Dependencies: B3, B4.

## Frontend lane (parallel on synthetic contract)

### F1: strict GPU boundary
- [ ] Add typed GPU payload parser and synthetic fixtures/tests for all states,
  exact keys, indices/cap, ranges, UTC timestamps, nulls and unsafe numbers.
- Acceptance: unknown/private fields and invalid combinations rejected; CPU
  validation remains unchanged; no new dependency.
- Verify: frontend unit/lint/typecheck/build.
- Files: web/src/gpu.ts and tests, optional synthetic helper file.
- Dependencies: contract frozen in spec.

### F2: one stream, independent freshness
- [ ] Add GPU named-event subscription to the existing EventSource and bounded
  per-sensor monotonic expiry state; use correctly keyed event mocks.
- Acceptance: CPU-only frames never refresh GPU, cached/replayed GPU frames
  never extend expiry, six-second stale states, removal frees device state,
  StrictMode/disconnect cleanup and single connection preserved.
- Verify: deterministic fake-clock/hook tests plus frontend quality checks.
- Files: useMetrics.ts, its tests, GPU state helper/tests; App test doubles if
  necessary in a separate compatibility commit.
- Dependencies: F1.

### F3: accessible current GPU cards
- [ ] Add GPU section with generic indices, utilization/VRAM/temperature/power,
  opt-in instructions and disabled/unsupported/unavailable/stale states.
- Acceptance: GiB/percent/Celsius/watts correct; no fabricated zeroes, external
  assets, identifying labels, added history/storage, or mutable controls.
- Verify: rendering/state/unit tests, lint/types/build, responsive demo browser.
- Files: GPU component/tests, App.tsx, styles.css; keep small and reviewable.
- Dependencies: F2.

### F4: production demo integration
- [ ] Extend demo browser coverage for --gpu and disabled mode, partial/failure
  states and freshness; preserve CPU/security/reconnect tests.
- Acceptance: Chromium passes against final embedded UI; all tests synthetic,
  no external requests, no real-driver constructors, no server reuse.
- Verify: full frontend/Go checks and Chromium. Attempt/report WebKit accurately;
  do not install missing system packages or suppress browser failures.
- Files: web/playwright.config.ts, e2e GPU/CPU/privacy tests; split if needed.
- Dependencies: F3 and built backend B5; test-supplied binary env allowed.

## Documentation, review and handoff

### D1: operator docs and exact notices
- [ ] Update README/API/security/contributor references and required NVML/header/
  transitive licenses from exact pinned upstream files, never generated prose.
- Acceptance: --gpu opt-in and failure behavior clear; power/memory semantics,
  physical-device limit and native-call cancellation limitation documented;
  header disclaimer retained in adapter comments and user docs.
- Verify: linked Linux graph, byte/normalized-license equality, links/fences/
  shell syntax, no operator/private data. Split any >5-file change.
- Dependencies: B3/B5 final manifest/runtime behavior; F3 labels.

### V1: independent evidence and integration
- [ ] Separate GPT review; fix accepted findings with one writer and reverify.
- [ ] Integrate reviewed commits; run native/no-CGO Go and frontend/browser/
  audit/secret/artifact checks; preserve owner's running server.
- [ ] Explicit live numeric GPU comparison without logging actual readings,
  load generation, process/identity APIs or host configuration changes.
- [ ] Update task/report evidence, disclose unavailable WebKit/Safari checks,
  commit clean state. No push/public deployment without explicit permission.
