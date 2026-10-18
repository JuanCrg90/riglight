# Tasks: Riglight milestone 1

Status: Approved — GPT-only Herdr implementation with isolated feature worktrees.
Tasks are dependency ordered; the frontend can use the fixed synthetic contract
in parallel with backend implementation before integration.
Each task is a bounded increment, normally 1–5 files including tests. Split any
task that exceeds five files or introduces another independent responsibility.
Write failing behavior tests before implementation. Stage/commit only verified
increments, preserving unrelated changes and all hooks.

Bootstrap tasks intentionally introduce tooling progressively; do not expose a
build/test script before its input files exist. Every introduced check must pass
before the increment is considered complete. Completion boxes are updated only
when the coordinator has verified the integrated result.

## 1. Pin local tooling

- [ ] Pin Go 1.27.2 and Node 24.22.0 in mise.toml; document install, local tool
  selection, and required C compiler for race checks without system changes.
- Acceptance: `mise exec -- go version` and `mise exec -- node --version` match;
  global tool selections remain unchanged; no service or privileged install.
- Verify: `mise install`, version checks, `command -v cc`, `git diff --check`.
- Files: mise.toml, README.md.
- Dependencies: none. Scope: small.

## 2. Establish sample and privacy contracts

- [ ] Add typed CPU/memory snapshots and strict allowlist serialization; add
  synthetic success, not-ready, and unavailable contract fixtures/tests.
- Acceptance: JSON exactly matches the plan; invalid/nonfinite values are
  rejected; NOT_READY/UNAVAILABLE serialize null values, not fabricated zeroes.
- Verify: `mise exec -- go test ./internal/metrics`, `mise exec -- go vet ./...`.
- Files: go.mod, internal/metrics/sample.go, internal/metrics/sample_test.go,
  docs/api.md.
- Dependencies: 1. Scope: medium. No gopsutil dependency yet.

## Checkpoint A: local contracts

- [ ] Check newly introduced Go code is gofmt-clean and tests/vet pass.
- [ ] Review the public allowlist and schema examples before consumers are added.

## 3. Prove synthetic sample fan-out

- [ ] Implement a shared latest snapshot, atomic initial subscription, and
  capacity-one subscriber queues, with one deterministic synthetic source.
- Acceptance: no per-subscriber collection; no stale-first-sample ordering;
  slow consumers do not block updates and unsubscribe is idempotent/race-free.
- Verify: `mise exec -- go test -race ./internal/metrics` with concurrency tests.
- Files: internal/metrics/hub.go, internal/metrics/hub_test.go,
  internal/metrics/demo.go, internal/metrics/demo_test.go.
- Dependencies: 2. Scope: medium.

## 4. Secure the HTTP boundary

- [ ] Add exact Host/Origin checks, response headers, method validation, and
  health/snapshot routes; no listener or live host collector is needed yet.
- Acceptance: foreign/malformed hosts, hostile/null origins, and mutable methods
  fail; allowed local GETs return the documented shape without private errors.
- Verify: `mise exec -- go test -race ./internal/server`, `mise exec -- go vet ./...`.
- Files: internal/server/security.go, internal/server/security_test.go,
  internal/server/server.go, internal/server/server_test.go.
- Dependencies: 3. Scope: medium.

## 5. Stream snapshots safely

- [ ] Add SSE framing, initial snapshot, flush/deadline handling, eight-client
  limit, and cleanup on disconnect/application cancellation.
- Acceptance: synthetic end-to-end stream receives updates; no network write can
  block publication; over-limit/method/host/origin failures allocate no subscriber.
- Verify: `mise exec -- go test -race ./internal/server ./internal/metrics`;
  deterministic slow-writer/deadline/disconnect/shutdown tests, no sleep-only proof.
- Files: internal/server/events.go, internal/server/events_test.go,
  internal/server/server.go, internal/server/server_test.go.
- Dependencies: 4. Scope: medium. Request independent security/concurrency review.

## Checkpoint B: synthetic transport

- [ ] Go tests/race/vet/format checks pass, including security/stream abuse cases.
- [ ] Review transport lifecycle evidence before adding live host access.

## 6. Bootstrap reviewed frontend dependencies

- [ ] Add exact dependency/tool versions, the web installation boundary and
  fail-closed dependency-script policy; review the resulting frozen lockfile.
- Acceptance: install succeeds with scripts blocked; engine/peer warnings are
  resolved, not suppressed; native dependency/script requirements are recorded.
- Verify: `pnpm --dir web install --ignore-scripts`, then a frozen install with
  the reviewed script policy; `pnpm --dir web audit` with reachability triage.
- Files: web/package.json, web/pnpm-workspace.yaml, web/pnpm-lock.yaml.
- Dependencies: 1. Scope: medium. Include only reviewed plan dependencies.

## 7. Render a static demo shell

- [ ] Add index, React entrypoint, minimal accessible App and TypeScript config;
  introduce build/typecheck scripts only with their working source inputs.
- Acceptance: a loopback development server renders clearly labeled synthetic
  CPU/RAM; frontend build and strict typecheck pass with no external assets.
- Verify: `pnpm --dir web build`, `pnpm --dir web typecheck` and a local browser
  smoke check; no host metrics or upstream requests.
- Files: web/index.html, web/src/main.tsx, web/src/App.tsx, web/tsconfig.json,
  web/package.json.
- Dependencies: 6. Scope: medium. Built-in Vite JSX handling is sufficient for
  this bootstrap; add the reviewed React plugin configuration in the next task.

## 8. Activate frontend quality tooling

- [ ] Configure the reviewed React plugin, jsdom/Vitest, strict ESLint checks,
  and explicit loopback Vite/API proxy behavior without disabling backend checks.
- Acceptance: frontend tests, lint, types, and build pass; Strict Mode is enabled;
  dev origin policy is explicit and production remains same-origin.
- Verify: `pnpm --dir web test -- --run`, `pnpm --dir web lint`,
  `pnpm --dir web typecheck`, `pnpm --dir web build`.
- Files: web/vite.config.ts, web/eslint.config.js, web/src/App.test.tsx,
  web/src/test-setup.ts, web/package.json.
- Dependencies: 5, 7. Scope: medium.

## Checkpoint C: frontend foundation

- [ ] Demonstrate the synthetic UI; tests/lint/types/build and Go checks pass.
- [ ] Review data transport and privacy decisions before connecting the browser.

## 9. Consume validated SSE data

- [ ] Add runtime sample validation and a single cleaned-up EventSource hook;
  replace the static App samples with the hook's live synthetic feed.
- Acceptance: invalid frames never advance freshness; reconnect/Strict Mode
  cycles leak no resources; there is no competing initial snapshot fetch.
- Verify: `pnpm --dir web test -- --run`, frontend lint/typecheck/build;
  unit tests for validation, reconnection, duplicate frames, and cleanup.
- Files: web/src/metrics.ts, web/src/metrics.test.ts, web/src/useMetrics.ts,
  web/src/useMetrics.test.tsx, web/src/App.tsx.
- Dependencies: 8. Scope: medium.

## 10. Serve the production demo

- [ ] Embed compiled assets and add a loopback-only CLI with demo selection,
  signals, collector lifecycle, stream cancellation, and bounded HTTP shutdown.
- Acceptance: a built executable serves UI/API/SSE; demo does not create a live
  collector; invalid listen addresses fail without leaking paths/configuration.
- Verify: frontend build; `mise exec -- go test -race ./...`,
  `mise exec -- go vet ./...`; `mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight`;
  `bin/riglight --demo` and local curl/browser checks.
- Files: web/embed.go, cmd/riglight/main.go, cmd/riglight/main_test.go,
  internal/server/assets.go, internal/server/assets_test.go.
- Dependencies: 9. Scope: medium. Ensure tests that need assets run after UI build.

## Checkpoint D: end-to-end demo

- [ ] Demonstrate real SSE updates from the production executable with synthetic
  data; confirm shutdown/disconnect behavior and no third-party runtime requests.
- [ ] Independent review covers HTTP/SSE/CLI security before live collection.

## 11. Collect real CPU/RAM

- [ ] Add reviewed gopsutil dependency and a single allowlisted Linux collector;
  inject collection dependencies to preserve deterministic synthetic tests.
- Acceptance: CPU baseline/deltas and memory totals are correct; per-metric
  failures are independent; demo never calls live metrics, even during startup.
- Verify: `mise exec -- go test -race ./...`, vet, format and production build;
  opt-in local live smoke check without saving metric output to public files.
- Files: go.mod, go.sum, internal/metrics/live.go, internal/metrics/live_test.go,
  cmd/riglight/main.go.
- Dependencies: 10. Scope: medium. Reconfirm gopsutil accounting from official
  source before implementing the delta formula; do not infer health from zeroes.

## 12. Show bounded history and recovery states

- [ ] Add 450-sample SVG sparklines and responsive styling; surface mode,
  connectivity, freshness, and independent unavailable states accessibly.
- Acceptance: old/duplicate samples and clock skew cannot fake freshness;
  unavailable intervals are gaps; no persistent storage or external requests.
- Verify: frontend test/lint/typecheck/build, deterministic fake-clock tests,
  keyboard/responsive/reduced-motion browser checks against demo mode.
- Files: web/src/App.tsx, web/src/App.test.tsx, web/src/Sparkline.tsx,
  web/src/styles.css, web/src/useMetrics.test.tsx.
- Dependencies: 11. Scope: medium. If hook changes are needed, split them into
  their own tested increment rather than exceeding this task's file limit.

## Checkpoint E: usable monitoring

- [ ] Demonstrate live CPU/RAM locally and demo recovery states without retaining
  personal output. All applicable backend/frontend checks pass.
- [ ] Review accessibility and freshness behavior before final browser coverage.

## 13. Automate browser/privacy smoke checks

- [ ] Configure Chromium/WebKit tests against production demo mode; check stream
  updates/reconnect, responsive layout, security headers and outbound requests.
- Acceptance: no real metrics, logs, external requests, or personal screenshots;
  demo never constructs a host collector; browser failures identify true blockers.
- Verify: `pnpm --dir web exec playwright install chromium webkit`,
  `pnpm --dir web test:e2e` plus unit/race checks after any accepted fixes.
- Files: web/playwright.config.ts, web/e2e/dashboard.spec.ts,
  web/e2e/privacy.spec.ts, web/package.json.
- Dependencies: 12. Scope: medium. No privileged browser-library installation;
  prefer playwright-cli for interactive verification, not Chrome DevTools MCP.

## 14. Document safe development and use

- [ ] Replace design-stage README with verified setup/build/run/check commands,
  same-port SSH tunneling, limitations, and contribution/security guidance.
- Acceptance: clean setup is reproducible; notices cover shipped dependencies;
  no public ingress, service enablement, Docker permissions, or real identifiers.
- Verify: execute documented commands; review examples, Git diff and licenses.
- Files: README.md, CONTRIBUTING.md, SECURITY.md, THIRD_PARTY_NOTICES.md,
  docs/spec.md.
- Dependencies: 13. Scope: medium. CI automation requires separate approval.

## Checkpoint F: completion gate

- [ ] Run the agreed format/tests/race/vet/build/UI/browser/audit checks.
- [ ] Run pinned govulncheck and Gitleaks; inspect assets/binary for local paths
  and review tracked content/history for personal data beyond scanner coverage.
- [ ] Independent review receives intent, diff and verification evidence; triage
  findings, apply accepted fixes with one writer, and reverify changed behavior.
- [ ] Record any missing tool, Mac/Safari verification, hardware, or advisory
  service as an explicit limitation. Do not claim skipped checks passed.
- [ ] Keep public publication, remote repository creation, service installation,
  GPU, Docker, and llama-swap outside this milestone without further approval.
