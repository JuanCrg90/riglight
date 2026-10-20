# Plan: optional system detail

Follow [the system-detail specification](../docs/spec-system.md). Owner chose root
filesystem only and configured uplinks. No live interface discovery/configuration,
app/helper restart, daemon/permission/service/network changes or publication.
Primary owns architecture/delegation/integration; GPT-only isolated worktrees.

## Contract and Go slices

- [ ] Independent plan/contract review, then tests-first typed contract and
  Validate/MarshalAt with per-field monotonic ages, disabled/demo/null/zero
  semantics. Files: internal/systemmetrics/sample.go + sample_test.go.
- [ ] Canonical hub/demo source and synthetic fixtures. Files: hub.go/demo.go +
  tests. Depends contract. Verify focused Go/race/serialization; no live calls.
- [ ] Injected production source and exact interface selection validation.
  Project only swap bytes, root used/available/total and selected RX/TX deltas.
  Files: live.go/config.go + tests (split if >5). Depends contract. Verify nil/
  failures/reserved-space/numeric boundaries, genuine zero, per-interface resets,
  missing/duplicate selections, sum overflow, nonuniform monotonic intervals and
  baseline recovery. All provider data synthetic; no host/network discovery.
- [ ] CLI --system-details + repeatable --network-interface, independent worker,
  disabled/demo no construction and shared shutdown deadline. Files: cmd/riglight
  main.go/system.go + tests. Depends source. Verify factory/CLI/slow-source tests.
- [ ] Optional server hub/snapshot and fifth named system SSE source using the
  existing writer/client slot/subscriptions/security controls. Files: server.go,
  events.go + tests. Verify all guards/single stream/cancel/race and other sources.
- Checkpoint: separate Go correctness/security review and accepted fixes before
  primary integrates. Go writer owns only required Go files, no docs/frontend.

## Frontend slices (parallel against frozen contract)

- [ ] Strict system DTO parser and per-sensor bounded freshness, six-second age,
  timer expiry only, min-expiry persistence, replay/mode/rollback boundaries.
  Files: system.ts/systemFreshness.ts + tests. Verify synthetic frontend tests.
- [ ] Existing hook's named system event/cleanup/mode binding (including disabled),
  no extra transport/fetch; tests. Depends parser. Files: hook + hook tests.
- [ ] Accessible compact System detail UI/App/styles/tests with swap/no-swap,
  root used/available/total, selected-interface receive/transmit units and honest
  disabled/pending/unavailable/stale values. No identities/charts/controls/export.
  Split component and hook commits; no Go/e2e/docs edits by frontend writer.
- Checkpoint: separate model/context UI review, fixes, frontend tests/lint/types/
  build then primary cherry-picks reviewed commits and rebuilds embedding assets.

## Primary integration and handoff

- [ ] Add synthetic Chromium/WebKit system project on 19094; fifth-source single
  EventSource, zero swap/idle rates, reserved disk space, partial/stale/replay/
  malformed/mode cleanup and responsive/read-only cases. Run all previous suites.
- [ ] Document CLI/configuration, numeric meanings, privacy and syscall-cancel
  limits in README/SECURITY/docs/api.md and system guide; no actual interface
  identifiers or readings. Live selection/enablement remains a later decision.
- [ ] Full pinned frontend frozen tests/lint/types/build, Go native/no-CGO tests/
  race/vet/format/build, both existing binaries, audits/module/link-license graph
  comparison, history/export/frontend/binary identifier/path/source-map scans.
  Report actual WebKit launch blockers, not skips or a Safari success claim.
- [ ] Independent combined final review, reverify fixes, record verification and
  task completion; clean verified commits, no running-app changes or push.

## Verification commands

```sh
mise exec -- pnpm --dir web test
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
```

Build frontend before Go embedding. Use fake collectors/clocks and test-owned
loopback listeners only. The system collector can block in an underlying root
statfs/proc syscall; one worker/no overlap and bounded shared shutdown isolate
that risk rather than spawning unbounded abandoned goroutines.
