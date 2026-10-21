# Follow-loaded-model log plan

Status: implemented, independently reviewed and verified; see the
[executed verification](../docs/verification-loaded-logs.md). Production log
activation, installed upstream compatibility and reboot remain untested/unmodified.

Owner clarified automatic following of the single currently loaded model.
Follow [the contract](../docs/spec-loaded-logs.md); preserve the existing log
privacy/audience and default-off/live-rollout boundaries. No production reads,
model controls, service changes or Safari requirement.

- [x] Independent contract review: fresh unique loaded target, expiry, one reader,
  serialized target/line transitions and session-global budgets.
- [x] Go writer: validated --log-loaded-model/XOR wiring, existing inventory-hub
  selection and monotonic freshness, optional target-aware provider/interface,
  serialized cancellation/join, shared input budget, strict target envelopes/demo.
  Own Go source/tests only; no helper behavior changes or dependencies.
- [x] UI writer: new catalog kind and exact target parser, automatic-source default,
  target-state/label display, no output before FOLLOWING, clear-on-change while
  preserving sequence guard, pause/resume/source/mode/session isolation and tests.
  Own web/src only; primary owns E2E/config/docs/integration.
- [x] Synthetic backend cases: A->B, A->none->B, unchanged A, stale without more
  publications, truncated/unavailable/mismatched/ambiguous/unsupported targets,
  no unintended I/O, aggregate limits, quiet cancellation, EOF and shutdown.
- [x] Independent GPT Go/security and frontend review; primary triage and fixes
  through one writer per file, then reverify.
- [x] Primary Chromium synthetic target-transition checks plus existing suites;
  no browser connection recreation for within-session model changes.
- [x] Document auto selection, fresh-unique rules, target envelopes and limits;
  complete pinned frontend/Go/race/native/no-CGO/build/advisory/privacy gates.
- [x] Record actual verification and clean commits; live activation remains a
  separate owner-approved deployment, not an implementation test.
