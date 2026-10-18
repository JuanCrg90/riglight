# Plan and tasks: llama-swap inventory

Status: authorized, capability investigation complete; implementation pending.
Contract: [model specification](../docs/spec-models.md).

The owner chose inventory/lifecycle only, no auth, configured loopback access.
Seven completed CPU/GPU workers and their panes were closed before this increment;
worktrees/history/unrelated work were preserved. Do not resurrect old tasks.

## Ordered slices

Backend lane (GPT-5.6 Terra high; bounded HTTP/security/lifecycle work):

- [ ] B1: Add internal/llamaswap sample types, validation, canonical hub wrapper,
  synthetic demo and serialization/copy/age tests. No upstream source yet.
- [ ] B2: Add literal-loopback URL validator and bounded read-only HTTP source;
  httptest fixtures cover only GET/v1/models, projections, statuses, unsafe IDs,
  duplicates/caps, redirects/proxies/TLS/timeout/cancel/body/header limits.
- [ ] B3: Add guarded GET/api/v1/models and models events to the same SSE writer;
  preserve CPU/GPU schemas, queues, deadlines, eight-client limit and nil config.
- [ ] B4: Add explicit CLI opt-in/default/custom URL/demo wiring and independent
  source worker. Prove no HTTP construction for disabled/demo; real CLI fake
  integration proves continued CPU/GPU/HTTP and bounded cancellation/shutdown.

Frontend lane (GPT-6 Luna medium; synthetic frozen contract):

- [ ] F1: Strict model DTO parser and synthetic boundary tests; no dependencies.
- [ ] F2: Third named listener in existing EventSource, inventory-specific
  monotonic expiry and bounded replay identities, clock rollback and cleanup.
- [ ] F3: Accessible compact aliases/coarse-state section with honest pending,
  disabled/empty/error/auth/unsupported/stale/truncated states and plain-text
  escaping. Preserve existing CPU/GPU views and scope-accurate introductory copy.
- [ ] F4: Synthetic production Chromium/WebKit model demo projects/cases;
  single connection, all three sources, error/replay/privacy/responsive checks.
  Wait for primary-provided integrated binary before claiming browser success.

Primary (only coordinator/architecture/integration authority):

- [ ] Review plan; separate independent GPT review of code and relevant fixes.
- [ ] Maintain operator/API/security/contributor documentation using targeted
  edits after reading originals. No dependency or license inventory changes
  expected; verify actual linked packages rather than inferring from go.mod.
- [ ] Integrate only reviewed/tested commits; build frontend first and rerun
  affected combined tests/race/vet/format/types/lint/native/no-CGO builds/browser.
- [ ] Passive configured-instance validation, no inference/controls/logs/process
  metadata, aliases/counts/raw bodies in reports. Stop all test-owned processes.
- [ ] Audits, exact notices, history/public-export/artifact privacy checks, status
  evidence and small verified commits. No push/publication without permission.

## Execution rules and acceptance

Each checkbox is split into compilable tested commits, normally <=5 files and
~100–300 changed lines. Tests first; record RED/GREEN, commands/results and
limitations. One writer per isolated worktree. Use the pinned mise toolchain;
Go assets must be built/copied before Go embedding checks. Worker scope excludes
primary docs/plans and other worktrees. Workers never delegate/control Herdr,
change architecture or contact the operator service. Ask the primary if blocked.

Reuse the canonical telemetry fan-out and SSE writer. Avoid duplicating a
concurrency hub or creating a generic plugin abstraction. New source is disabled
by default, independent of CPU/GPU, strict at both upstream and browser
boundaries, constant-memory bounded, and returns only projected aliases/states.

Accept only exactly supported Loaded/Unloaded/Unknown labels: /v1/models does
not prove ready. /running deliberately exposes command metadata, and general
upstream SSE exposes in-flight/configuration activity; neither is authorized.
No logs or other endpoints, even if they would provide richer lifecycle detail.

Missing native WebKit libraries remain an explicit environmental limit; no
system packages may be installed. Existing server state is the owner's concern;
do not start/stop/restart it as part of this increment. Tests use their own
loopback processes/ports and no synthetic stress workloads.
