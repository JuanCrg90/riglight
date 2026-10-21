# Log viewer implementation plan

Follow [the approved-intent contract](../docs/spec-logs.md). Owner chose the
existing admitted audience and llama-swap proxy diagnostics plus selected model
logs; selected Docker monitoring bounds remain. No actual log reads, live service
changes, installs, permission changes, persistence/export or publication. Safari
validation is explicitly excluded, not passed. All fixtures are synthetic.

## 1. Contract and adversarial review

- [x] Record source/audience choices and fixed official upstream evidence.
- [ ] Independent contract review for new sensitive-text boundary, safe defaults,
  bounded streaming/backpressure, exact source mapping and metric independence.
- [ ] Correct accepted issues before implementation; keep protocol/CLI synchronized.

## 2. Go writer: protocol and bounded source readers

- [ ] Define internal/logs DTO/catalog/status/limits and injectable providers.
  Strict typed parsing, fixed source IDs, per-session sequence, line/byte/time
  bounds, invalid UTF-8/control handling and deterministic demo cases.
- [ ] Add llama-swap fixed proxy/explicit model stream reader, canonical loopback
  URL reuse, safe references/reserved channel rejection, response/header/body
  bounds, redirects/proxies disabled and cancellation/quiet/unsupported tests.
- [ ] Add helper-only selected inspect identity/TTY projection and v1.44 logs
  allowlist, framed/TTY readers and adversarial fake Unix-daemon tests. Independent
  transport/session slots, no privileged dependency in dashboard link graph.
- Verify/commit each bounded slice: focused race/tests/vet/gofmt; no live queries.

## 3. Go writer: API, CLI and lifecycle

- [ ] Helper --logs opt-in, exact loopback catalog/stream guards, safe catalog and
  typed bounded SSE; no logs collected with flag absent or without a subscriber.
- [ ] Dashboard --logs/--llama-log-proxy/--log-model wiring with validated option
  combinations; separate guarded catalog/manual log stream routes and slot limits.
- [ ] Ensure source switch/EOF/failure/limits/disconnect/shutdown release every
  request/body/slot without starving existing five metric sources or stop budget.
- Verify: protocol/CLI/HTTP security/resource tests including cold/quiet/slow/
  malformed sources, parallel metrics and full native/no-CGO tests/builds.

## 4. Frontend writer: explicit inspector

- [ ] Strict catalog/log/status parsers with exact-key/body/text/byte/sequence/
  source/session validation; unknown content never reaches trusted markup.
- [ ] Manual catalog/source selection/start, warning, one EventSource, no automatic
  retry; bounded 200-line/256-KiB ring, pause/resume/clear/close/unmount semantics.
- [ ] Local filter, follow-tail, escaped content and accessible responsive controls.
  Source changes clear retained text/filter and never start a replacement implicitly.
- Verify/commit slices: frontend tests/lint/typecheck/build with synthetic events,
  memory bounds and hostile text; no new dependencies or actual source access.

## 5. Integration and independent review

- [ ] Primary synthetic Chromium log E2E project on a separate test-owned loopback
  listener; test closed/default behavior, manual start/stream/pause/close/filter,
  bounded/hostile states, no external fetch/storage and telemetry independence.
- [ ] Independent GPT Go/security review and UI/protocol review; primary triage and
  one writer per file for fixes, then reverify. Do not rely on scanner/reviewer alone.
- [ ] Document API/source privacy exception, exact flags, source setup, stream
  limits and no guaranteed redaction; update README/SECURITY/AGENTS intentionally.

## 6. Final gates and handoff

- [ ] Frozen frontend install/tests/lint/types/build before Go embedding; native
  and no-CGO tests/builds, race/vet/gofmt/module checks, Chromium suites and built
  helper fixture integration. No Safari/WebKit requirement or compatibility claim.
- [ ] Dependency/license/linked graph review and advisory checks; public-history,
  export/source and frontend/binary privacy scans. Real config/logs stay ignored.
- [ ] Verification record with exact checks, limits and failure history; clean
  commits. Nothing pushed or published; live log activation remains a later
  separately approved configuration/deployment step, not part of these tests.
