# Optional system-detail verification

## Delivered scope

Implemented opt-in --system-details: swap used/total, fixed root-filesystem
used/available/total capacity and aggregate RX/TX rates for 1–8 explicitly
configured uplinks. No process, device, interface, address, MAC, mount/type,
inode, raw-counter, disk-I/O, log, control, history or export fields are exposed.
No new dependencies or service/network/permission changes were introduced.

The owner chose root only and selected uplinks rather than all-interface totals.
Actual interface discovery/selection, live accuracy checks and enabling the
feature on the running dashboard remain separate later steps. No live system
providers were called for verification; all provider/clock/browser fixtures were
synthetic. Existing foreground dashboard/helper were not restarted/reconfigured.

## Verification results

- Frozen offline frontend installation with scripts blocked passed. **169
  frontend tests**, lint, strict typecheck and production build passed.
- Go module verification, tests/race/vet/gofmt and native/no-CGO tests/builds
  passed for the app and existing helper. The Go writer additionally verified
  the non-Linux demo cross-build. Assets were built before Go embedding.
- All Chromium projects: **35 passed**, including nine new system-detail cases.
  They cover real rendered values from the fifth named source on one EventSource,
  no browser snapshot fetch/external requests, read-only responsive UI, genuine
  zero/no-swap, reserved root capacity, independent failures/staleness, retained
  replay/shortened expiry, disabled-mode binding and distinct malformed-frame
  rejection. Existing CPU/GPU/models/containers tests passed.
- First combined browser run exposed an old GPU test counting all page dashes:
  the disabled system section legitimately added placeholders. Scoped those
  assertions to the GPU region (without weakening GPU expectations); all 35
  cases passed on the subsequent run. No application behavior was hidden/skipped.
- WebKit: **33 browser-launch failures before test bodies** due to missing
  libevent-2.1-7t64/libavif16; **two request-only cases passed**. No packages
  installed, tests skipped or new Safari compatibility claim made.
- Pinned govulncheck and pnpm audit reported no known vulnerabilities. Linked
  package comparison adds only the existing gopsutil/net package and Riglight's
  systemmetrics package; no new external module/license family. Dependency and
  notice manifests remain unchanged.

## Reviewed correctness and isolation

Separate GPT writers in isolated worktrees, independent GPT frontend/backend
reviewers and primary review covered the contract, numeric calculations,
privacy projection, lifecycle and combined evidence. Accepted fixes include:

- Exact all-null non-AVAILABLE sensor values, including partial pointer sets.
- Checked raw swap-unit/statfs-block arithmetic before subtraction/multiplication;
  invalid zero units are not assumed to be bytes. Reserved-space semantics and
  genuine zero swap remain distinct from failed/unsupported collection.
- Per-interface baselines so one reset cannot hide behind another interface's
  growth, checked aggregate deltas, missing/duplicate selected rows, actual
  monotonic elapsed time, overflow/idle/recovery tests and capability mapping.
- Cancellation checks before/between provider calls and before publication,
  fresh actual attempt timestamps rather than buffered ticker timestamps.
  The single worker does not overlap; an uncancelable syscall can stale system
  fields but cannot stall other workers/HTTP or extend shared shutdown.
- Strict browser key/mode/null/numeric validation, 16-KiB preparse cap, per-sensor
  32-identity LRU histories, persisted shortened expiry and timer-only expiration.
  Replay guarantees are bounded by retained identities, not unbounded memory.
- Tiny nonzero rates retain a nonzero indicator instead of rendering idle zero;
  disabled/pending states explain setup. No new fetch, stream, history or controls.

Final docs links/fences/shell command checks and full-history/public-export/
frontend Gitleaks scans passed. Public source/export and frontend/four-binary
operator-identifier, checkout-path and source-map inspections passed. Scanners do
not replace synthetic-data/field-allowlist review.
Nothing is pushed/published or installed as a service.

## Changed areas and remaining limits

- internal/systemmetrics: typed contract/hub/demo, raw Linux adapters, exact
  selection and checked live projector with synthetic tests/platform stubs.
- cmd/riglight and internal/server: optional independent worker/CLI, protected
  /api/v1/system and fifth named system event using existing admission/stream cap.
- web/src: strict parser/freshness/hook/accessible section; synthetic browser
  project/configuration and narrowly scoped preexisting GPU test assertions.
- Targeted API/security/operator docs, specification and completed task plan.

No live host values, interface identities, accuracy measurements, systemd runtime,
remote system-detail browser or boot verification is claimed. To enable later,
privately select uplinks and explicitly approve the dashboard restart. See
[setup and semantics](system-detail.md), [contract](spec-system.md),
[API](api.md) and [tasks](../tasks/system-plan.md).
