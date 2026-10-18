# GPU increment verification

The optional read-only NVIDIA GPU increment is implemented. Live collection is
explicitly enabled with `--gpu`; default collection is disabled and
`--demo --gpu` remains wholly synthetic. CPU/RAM wire data is unchanged.

## Completed checks

| Area | Result |
| --- | --- |
| Frontend unit tests | 73 passed |
| Frontend lint, TypeScript and production build | Passed |
| Frozen dependency install, lifecycle scripts blocked | Passed |
| Chromium production tests | 9 passed: four existing CPU/security/reconnect cases and five GPU cases |
| WebKit and WebKit GPU projects | Two API cases passed; seven browser cases could not launch because native libraries are missing |
| Go tests | All packages passed with the race detector |
| Go vet and gofmt | Passed; formatting check empty |
| Native and no-CGO builds | Passed with `-trimpath` |
| No-CGO tests | All packages passed |
| Go module verification and govulncheck | Passed; no vulnerabilities found |
| pnpm advisory audit | No known vulnerabilities found |
| Dependency notices | NVIDIA license/header copied exactly; complete header notice also retained in adapter comments |
| Privacy checks | History secret scan and tracked-source/build personal-path/source-map checks passed |

Primary reran the combined frontend/Chromium and native/no-CGO Go checks after
integration. The independent GPT reviewer inspected the source, tests and fixes;
review approval is not presented as another test run. Browser fixtures and
responses are synthetic; automated traces/screenshots/video are disabled.
A temporary synthetic-only desktop capture was visually reviewed; stale
milestone/CPU-only introductory copy was corrected with a regression test.
Test-owned listeners use separate loopback ports with server reuse disabled.

Changed areas: `cmd/riglight`, `internal/gpu`, the shared `internal/telemetry`
hub and CPU wrapper, `internal/server`, frontend GPU parsing/state/cards/tests,
Go dependency pins, operator/API/security docs and exact upstream notices.
Commands run through the pinned mise environment include:

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
mise exec -- pnpm --dir web exec playwright test --project=chromium --project=chromium-gpu --workers=2
mise exec -- pnpm --dir web exec playwright test --project=webkit --project=webkit-gpu
mise exec -- pnpm --dir web audit --audit-level low
```

The pinned govulncheck and Gitleaks tools were also run; temporary tool paths
and private validation helpers are intentionally not published.

## Passive live reference comparison

A separate test-owned loopback instance enabled native GPU collection. Eight
GPU frames were compared with numeric-only `nvidia-smi` queries; no names,
UUIDs, serials, PCI information, processes, driver versions or raw diagnostics
were queried or retained. The original running server was not restarted.

- VRAM used/total matched the CLI within two MiB of integer-MiB rounding.
- Temperature comparisons were within three Celsius degrees.
- At least 75% of utilization comparisons were within ten percentage points.
- At least 75% of power comparisons were within the greater of ten watts or
  15% of the reference. These tolerances accommodate independently timed samples
  and driver averaging; they are not a claim of synchronized instrument accuracy.
- Available sensor frames remained fresh.

A simultaneous independent `/proc/stat` and `/proc/meminfo` comparison confirmed
CPU/RAM remained accurate in the GPU-enabled build: ten aligned CPU intervals,
eleven RAM comparisons, exact RAM total, and the expected two-second cadence.
Only comparison aggregates/booleans were retained privately; this report contains
no actual readings, inventory, host identities or screenshots. No artificial
CPU/GPU load or host configuration changes were used.

## Failure and lifecycle coverage

Synthetic tests cover partial/unsupported/null sensor readings, range checks,
strict payload fields, inventory cap/truncation, cached monotonic age, duplicate
replay, clock rollback, default/demo isolation and ownership/copy boundaries.
Injected CLI integration proves a blocked GPU poll does not stop CPU publication
or loopback HTTP, shutdown respects the shared five-second budget, and native
cleanup does not race a still-running call. Failed initialization is attempted
once per process and requires restart to retry.

Runtime smoke checks also passed for default-disabled GPU collection, synthetic
GPU data in a no-CGO build, and no-CGO live CPU/RAM with an explicit unsupported
GPU backend. All test-owned processes were stopped; the original server remained
healthy. One preliminary smoke used an incorrect CPU endpoint and was corrected
to the documented `/api/v1/snapshot` before rerunning successfully.

## Limits and distribution

WebKit requires `libevent-2.1-7t64` and `libavif16` on this test host; they were not
installed. The WebKit GPU project exists and its tests were attempted, not
skipped. Actual Safari on a Mac remains unverified.

Native driver calls cannot be interrupted safely. CPU/HTTP collection is
isolated, but shutdown cannot force a hung call to finish; it does not unload
the library concurrently. Physical-device telemetry only; MIG instance/process
monitoring, GPU history, alerts and control are not implemented.

The linked Linux production module graph was inspected rather than inferred
from `go.mod`: NVIDIA/go-nvml, gopsutil, go-sysconf, numcpus and x/sys. The indirect
purego module is not linked in this profile. Other build targets require their
own dependency-notice inventory before redistribution. Preserve the MIT license,
third-party notices and referenced license files when distributing. Nothing was
pushed, published or installed as a service.
