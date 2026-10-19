# Selected Docker monitoring verification

## Delivered scope

Implemented optional selected-container monitoring, a separate filtering-helper
executable, the projected HTTP/SSE contract and accessible dashboard view. The
helper alone accepts explicitly configured Unix-socket access and alias/reference
selection. The dashboard gets no Docker socket or raw inspect response. No new
dependencies, CI, telemetry, logs, controls, permissions, services or deployment.

The owner chose full state/health/restart/CPU/RAM monitoring through the filtering
helper rather than inventory-only delivery. All daemon/proxy/browser test data
was synthetic. No real Docker daemon was contacted, no helper was deployed and
the owner's running tailnet dashboard was not restarted or reconfigured. Live
engine compatibility, container selection and deployment remain separate work.

## Commands and results

Run through the pinned mise toolchain, frontend before Go embedding:

- Frozen offline pnpm install with scripts blocked: passed.
- Frontend tests: **153 passed**; lint, strict types and production build passed.
- Go module verification, go test -race ./..., go vet ./..., gofmt and diff checks:
  passed. Native and CGO_ENABLED=0 tests/builds passed for both executables.
- Full Chromium projects: **26 passed**, including nine new container cases.
  Coverage includes four named sources on one EventSource, local-only requests,
  read-only/responsive layout, missing/unsupported/disabled/auth states,
  independent metadata expiry, maximum-16-row freshness, cache/shortening and
  non-available timestamp replays, invalid aliases and extra-field rejection.
- WebKit projects: **24 browser launches failed before test bodies ran** because
  libevent-2.1-7t64 and libavif16 are missing; **two request-only cases passed**.
  No packages were installed, failed cases skipped or launch failures hidden.
  These results are not a Safari or Docker-view-on-Mac compatibility claim.
- Pinned govulncheck and pnpm audit: no known vulnerabilities reported.
- Dependency/notice manifests remain unchanged; the main executable's linked
  package graph excludes internal/dockerproxy. Full-history, public-export and
  frontend Gitleaks scans passed. Public source and all four native/no-CGO
  binaries/frontend assets passed operator-identifier, checkout-path and source-
  map inspection. No scans substitute for the synthetic-data/privacy review.
  Nothing was pushed or published.

## Independent review and corrections

GPT-only isolated backend/frontend writers were reviewed by separate contexts
and different GPT implementations. Primary inspected code/evidence and accepted
fixes, rather than treating reviewer approval as a test rerun.

- Plan review clarified the hard poll cancellation budget and fair rotation,
  memory-counter precedence/equality, exact name/ID binding and trusted-local-
  process loopback boundary (Host/Origin guards are not authentication).
- Backend review/primary inspection fixed JSON null becoming numeric zero,
  invalid CPU-count fallback, per-row memory failure isolating other rows,
  non-available DTO values, strict hex IDs, any-Origin/empty-query rejection,
  unstarted-row baseline clearing, age-plus-request-duration overflow, body
  timeout classification, UTC validation and valid non-available proxy snapshots.
- Frontend review/primary inspection replaced one shared 32-entry cache with
  bounded per-overview/per-field identity histories, pure timer expiration and
  persisted shortened expiries. Fixed missing/pending metadata mislabeling,
  disabled-mode binding and non-available timestamp replay poisoning.
- Regressions and final independent backend/frontend reviews passed. Reviewers
  disclosed which commands they reran and which integration checks were primary-
  only. No review worker queried actual services or retained real page data.

## Synthetic executable proof

A test-owned temporary Unix HTTP daemon returned an inspect response containing
synthetic configuration, identity and healthcheck-output sentinels, plus known
one-shot counters. The built helper made only the two selected pinned GET paths.
Its projected response excluded IDs, reference names and every sentinel, while
correctly reporting state, health, restarts, CPU delta and cache-adjusted memory.
Wrong/empty Origin, foreign Host, POST and a trailing empty query were rejected.
The temporary helper/daemon were stopped; this was not a real-daemon test.

The existing dashboard's interface-restricted listener and Tailscale Serve
fingerprint were checked privately without saving actual readings/identifiers to
public artifacts. Synthetic checks and configured fake sources do not establish
compatibility with an installed Docker Engine or replace live operator review.

## Files and remaining limits

- internal/containers: contracts/hub/demo/projected HTTP source and tests.
- internal/dockerproxy and cmd/riglight-docker-proxy: selection, filtering,
  bounded collection, machine API/lifecycle and tests.
- cmd/riglight and internal/server: optional independent worker/route/SSE wiring.
- web/src: validator/freshness/single-hook/section; synthetic container E2E and
  project configuration; targeted operator/API/security docs and task completion.

No daemon discovery/version probe, root/group grant, raw proxy, log endpoint,
Docker mutation, image/config collection in the public DTO, service installation,
owner-app restart or publication was performed. See [setup](docker-access.md),
[specification](spec-docker.md) and [tasks](../tasks/docker-plan.md). The helper
necessarily receives bounded inspect data transiently to project required fields;
unknown config/healthcheck output is discarded, not forwarded, persisted or logged.
