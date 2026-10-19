# Implementation plan: selected Docker monitoring

Follow [the approved feature specification](../docs/spec-docker.md).
Owner chose selected containers and a filtering helper with full state/health,
restart and CPU/RAM monitoring. Implement/test/prepare instructions only; no
actual daemon connection, helper deployment, permissions or service changes.
Keep the existing tailnet dashboard running untouched.

## Decisions and dependencies

- A GET-only pass-through proxy can expose secrets from inspect. A separate
  stdlib filtering executable is the privileged boundary, not the dashboard.
- Selection is explicit at the helper; alias-to-exact-reference mapping never
  comes from browsers. The main source consumes only a bounded projected DTO.
- Contract -> synthetic sample path -> helper source -> main source -> one SSE
  and accessible UI -> adversarial integration/review. No SDK/new dependencies.
- Freeze JSON/enums/freshness in spec before parallel work. Use isolated GPT-only
  Herdr worktrees, one writer per file, primary owns architecture/integration.
- Backend writer owns all Go source/tests needed for this increment; frontend
  writer exclusively owns new container UI/contracts and existing web/src hook/
  App changes. Primary owns docs/tasks and browser configuration/spec additions.
  No worker delegates, changes architecture, uses real services or deploys.

## Bounded tasks (test before implementation)

### A. Public contract and synthetic source
- [ ] Allowlisted typed sample/validation/MarshalAt; arrays/nonavailable nulls,
  sorted unique aliases, safe bounds and independent metadata/CPU/memory ages.
  Files: internal/containers/sample.go + sample_test.go.
  Verify: go test ./internal/containers, vet/gofmt.
- [ ] Canonical hub and deterministic synthetic publisher; no live construction.
  Files: internal/containers/hub.go, demo.go + tests. Depends A.
  Verify: focused Go tests/race, serialization fixtures.
- Checkpoint: contract tests and independent plan/security review pass.

### B. Filtering helper (risk-first)
- [ ] Validate aliases/references and Unix transport; fixed v1.44 inspect path,
  bounded typed projection and generic errors, ID/name binding. No list/logs.
  Files: internal/dockerproxy/config.go, source.go + tests. Depends A.
  Verify: fake Unix daemon exact method/path/body/bounds/selection/sentinels.
- [ ] One-shot stats and per-ID CPU baselines, cache-adjusted memory, four-way
  acquisition/poll limits, independent ages/failed states. Files: collector.go,
  collector_test.go and stats.go/stats_test.go. Depends previous task.
  Verify: zero/reset/recreation/count-change/cgroups/timeout/cancel/race tests.
- [ ] Loopback cached snapshot HTTP boundary and helper lifecycle, fixed five-
  second cadence and one total shutdown budget. Files: server.go + tests and
  cmd/riglight-docker-proxy/main.go + tests. Depends previous tasks.
  Verify: host/origin/input/method/security/cleanup tests, fake-socket process
  integration; no real Docker or privileged access. Separate security review.
- Checkpoint: helper outputs only allowlisted data; main binary lacks a Docker
  socket path/transport; all adversarial source and lifecycle tests pass.

### C. Dashboard's optional source and transport
- [ ] Literal-loopback projected HTTP source with strict DTO validation, incoming
  age retention plus request duration, deadlines/redirect/proxy/body/header bounds.
  Files: internal/containers/source.go + tests. Depends A/B contract.
  Verify: fake proxy cached replay/elapsed/skew/auth/bad DTO/oversize/cancel tests.
- [ ] --docker / explicit live --docker-proxy-url, independent worker, disabled/
  demo nonconstruction and idle cleanup. Files: cmd/riglight/main.go,
  containers.go + tests. Depends previous task/B.
  Verify: Go CLI/factory matrix, slow/failing source cannot stall other metrics.
- [ ] Optional container hub/route and named containers event on existing SSE.
  Files: internal/server/server.go, events.go + focused tests. Depends A/C.
  Verify: one writer/eight slots/four sources, protected route/stream, cancel/race.

### D. Frontend (parallel against frozen contract)
- [ ] Strict TypeScript DTO validator and bounded monotonic freshness helpers.
  Files: web/src/containers.ts, containerFreshness.ts + tests. Depends spec A.
  Verify: frontend tests/lint/types/build; cached replay/A-B-A/unavailable/mode/
  clock rollback and independent metadata/sensor expiry, cap/aliases/bounds.
- [ ] Add named event to existing hook and render accessible compact section;
  no new transport or history, controls/links/export. Files: hook + tests,
  App.tsx, ContainerSection.tsx + tests; split hook/section commits. Depends
  previous task; styles only if required and exclusively assigned.
  Verify: frontend tests/lint/types/build and synthetic keyboard/mobile checks.
- Checkpoint: independent backend/frontend reviews, then primary cherry-picks
  verified commits and rebuilds frontend before any embedded Go check.

### E. Integration, evidence and handoff
- [ ] Add synthetic Chromium/WebKit containers project at test-owned 19093,
  fourth-event/one EventSource, normal/partial/missing/error/unsupported/stale/
  invalid/replay cases. Files: e2e/containers.spec.ts, playwright.config.ts.
  Verify: existing suites and new cases; report missing WebKit native libraries.
- [ ] Operator/helper setup examples and threat boundary, no auto-install/live
  claims; update README/SECURITY/docs/api.md and a docker access guide in small
  documentation commits. Prepare private-config examples with synthetic aliases/
  references only. No actual socket/daemon inspection to fill configuration.
- [ ] Full native/no-CGO tests/race/vet/gofmt/build both executables, frozen web
  install/test/lint/types/build, module/advisory/audit/unchanged-license checks;
  history/export/frontend/binary/source-map/privacy scans; independent final
  review and reverify accepted changes. Record docs/verification-docker.md.
- [ ] Mark complete only after primary verification; keep known browser/deployment
  limits explicit. Commit small verified changes; no push/publication or app
  restart. Ask separately before live helper configuration/deployment.

## Risks

| Risk | Mitigation |
| --- | --- |
| Inspect contains secrets and healthcheck logs | Separate projection boundary, small typed DTO, no raw forwards/logs, sentinels |
| Privileged socket | Helper only, explicit existing access; no installs/root/group grants |
| Resource amplification or blocked daemon | Background bounded polling, four acquisitions, deadlines/body limits/cache |
| Wrong/recreated target | Exact reference validation and returned ID/name binding, stats by full ID, baseline resets |
| Misleading CPU/RAM/health | One-core CPU basis, cache counters/limits, unknown/unsupported/null states |
| Cached proxy success fakes freshness | Retain monotonic upstream ages, add request duration, bounded replay identity |
| More tailnet readers see private aliases | Owner-selected aliases only; same exact Host/Origin/admission/stream controls |
| False live/Safari claims | Synthetic fake daemon/proxy only; unchanged WebKit limitations; deployment deferred |
