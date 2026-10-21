# Ephemeral log-viewer verification

## Delivered scope and approval

The owner requested both viewers, chose the dashboard's existing admitted
all-tailnet audience, and selected llama-swap proxy diagnostics plus explicitly
selected model-server logs. Docker selection remains the helper's configured
containers. Combined/all-upstream/HTTP-access streams are excluded. Raw selected
text may contain secrets/prompts/configuration; there is no redaction guarantee.
The owner explicitly excluded Safari/WebKit validation for this increment.

Implemented default-off --logs, separate helper opt-in, exact source catalog,
manual warning/start/pause/resume/filter/follow/clear/close, separate bounded SSE,
strict source/mode/session binding, and no persistence/export. No production log
was read, no real log text/configuration was retained in public evidence, and no
installed service/binary/configuration, account/permission or network policy was
changed. Live source selection/activation is a later separately approved rollout.

## Verification results

Full primary pipeline passed after integration and the security patch:

- Frozen offline frontend install with dependency scripts blocked; **191 frontend
  tests**, lint, strict typecheck and production build passed.
- Assets built before Go embedding; module verification, gofmt, full race tests,
  vet and native/no-CGO tests/builds for dashboard and helper passed.
- **44 Chromium checks passed**, including nine new log cases. Real synthetic
  Go demo exercises manual source opening and the unchanged five metric events
  on one telemetry stream. Controlled synthetic browser events cover disabled
  catalogs, escaped hostile text/no external requests, filter/close, both UTF-8
  byte and line caps, pause-retain/resume/new-session keys, stale callbacks,
  terminal closure/no reconnect, foreign modes and oversized catalogs. API cases
  cover Host/Origin/method/query/replay/encoded/unknown-source rejection.
- Built helper against a test-owned fake Unix daemon passed disabled/live
  catalog selection, exact pinned inspect/stats/log GET allowlist, framed/TTY
  logs with split UTF-8 and independent stdout/stderr lines, no inspect-config
  sentinel forwarding, guards and bounded shutdown. No real Docker access.
- Standalone and reviewer race tests cover blocked/quiet/slow sources, line/input/
  frame/session caps, full-ID/TTY validation, cancellation, release/admission,
  malformed/missing/duplicate/null helper fields and metric independence.
- Linked package review found no new external module/license family and no
  privileged Docker package in the dashboard graph. App dependency versions,
  lockfile and legal notices unchanged. Go's license is byte-identical.

The fresh native cgo cache prints upstream go-nvml deprecated-binding warnings.
Builds/tests still succeed; warnings were not suppressed and no vendor/NVML API
or legal-notice changes were made. These bindings were preexisting dependencies.

## Failures found and fixed before handoff

Independent GPT protocol review required explicit raw-byte accounting (including
framing/discarded input), exact 64-lowercase-hex Docker IDs and catalog bounds.
A separate Go/security reviewer and UI reviewer reviewed isolated writers;
primary additionally audited cancellation, strict schema and UI lifecycle.
Accepted fixes include:

- Exact helper presence/type/duplicate/null validation, source IDs, required
  terminal SSE status, incomplete frames and catalog-mode/source binding.
- Empty catalogs retain [] rather than null; exact per-kind quotas and aliases.
- Absolute opening and complete catalog/inspect body deadlines, not just socket
  headers; independent helper log transport and prompt application cancellation.
- Dashboard admission before catalog/provider I/O, unknown helper aliases rejected
  before daemon I/O, and the helper's own 4096-line amplification limit.
- Dot references rejected before URL construction; no broader normalized paths.
- Pure React buffer accounting under StrictMode; no mutation inside state
  updaters. Bounded streaming catalog reader with timeout/abort, disabled-versus-
  unavailable states, primitive enums/reasons, first-sequence validation.
- Pause-retained lines survive Resume with unique session keys; old catalog/event
  callbacks cannot replace new sources. Retained output keeps its original mode
  label after a refreshed catalog changes mode. Follow affects only the log pane.

One review finding about missing empty-query rejection was withdrawn after the
primary verified the exact commit already checks ForceQuery on both routes.
No redundant workaround or weakened assertion was introduced for that finding.

Initial primary browser integration had eight locator failures: exact getByLabel
included select-option content, although the accessible combobox name was correct.
Changed to the exact combobox role/name. A subsequent buffer assertion matched
line-148 as line-1 because Playwright normalizes trailing whitespace; replaced it
with an anchored regex. The rendered byte/line bounds were already correct. All
44 cases passed in the final complete rerun; no failing case was skipped.

## Go security patch

The initial otherwise passing pipeline stopped at govulncheck with
**GO-2026-6809 / CVE-2026-111943**, “Distrust compromised intermediate certificates
in crypto/x509.” Official Go vulnerability data and issue 82345 identify 1.27.3
as the fixed 1.27 release. Updated both mise's exact toolchain pin AND go.mod's
minimum from 1.27.2 to **1.27.3**; no new dependency family or automatic installed
service upgrade. Reran the complete frontend/Go/helper/browser pipeline under
Go 1.27.3. govulncheck and pnpm audit then reported no known vulnerabilities.
Historical verification records correctly retain their older tested versions.

Sources:
- https://pkg.go.dev/vuln/GO-2026-6809
- https://github.com/golang/go/issues/82345
- https://github.com/golang/vulndb/blob/master/data/osv/GO-2026-6809.json

## Privacy scan triage

Gitleaks initially flagged one ordinary test-coverage sentence in the first spec
commit as a generic credential across a line break. The primary verified the
immutable source text contains no credential; current wording is clarified.
.gitleaksignore records only that one exact commit/file/rule/line fingerprint,
not a rule disablement or file-wide exclusion. Independent review confirmed this
classification and approved the narrow fingerprint exception. No actual secret
was ignored and no history was rewritten.

## Docker media-type compatibility

Docker API 1.42 and later uses application/vnd.docker.multiplexed-stream for
non-TTY container logs, while TTY logs retain the raw-stream type. See the
[official Moby v28.5.1 handler](https://github.com/moby/moby/blob/v28.5.1/api/server/router/container/container_routes.go#L187-L197).
The v1.44 helper now accepts that media type only when the selected container's
validated inspect TTY flag is false. A multiplexed header with TTY=true is rejected
before readiness/output. Existing raw-stream/octet-stream compatibility and
strict stdcopy/UTF-8/identity/input/cancellation bounds remain unchanged; framing
is never guessed from bytes or broadened to arbitrary response types.

Regression tests exercise multiplexed non-TTY output, TTY mismatch rejection and
MIME parameter/invalid-header cases. The built-helper fake Unix-daemon fixture now
uses the API's multiplexed type for non-TTY and raw-stream for TTY. It demonstrated
the old binary's rejection before rebuilding and passed with the corrected helper.
Post-fix full verification passed: 197 frontend tests, 50 Chromium checks, complete
Go race/vet/gofmt/module checks, native/no-CGO tests and four builds, six public
systemd tests, the built-helper fixture and vulnerability audits. No dependency,
selection or permission changes.

## Handoff and limitations

Final documentation/link/fence/shell checks, full-history/public-export/frontend
Gitleaks scans and complete source/export/frontend/four-binary operator-identifier,
checkout-path and source-map inspections passed. Scanners never replace reviewing
synthetic fixtures and the raw-content boundary.
No pushing/publication, service restart/activation or actual upstream compatibility
check is implied. New code and local binaries use Go 1.27.3; running installed
services remain on their previously deployed binaries until separately upgraded.

- Single-segment safe model references only; slash-containing IDs are deliberately
  rejected, not automatically rewritten or probed.
- Quiet upstreams that do not flush headers can time out honestly. Sessions end
  at hard limits and do not automatically reconnect; reopening may repeat history.
- Source selection is explicit; a Docker session stays on its opening instance.
- Logs are NOT reliable redaction, freshness/readiness evidence, persistent
  history, authentication or an export feature.
- No Safari/WebKit or reboot compatibility claim; neither was required/exercised.

See [setup/privacy](log-viewers.md), [contract](spec-logs.md),
[API](api.md) and [tasks](../tasks/logs-plan.md).
