# Optional log viewers

Log viewers are an explicit, read-only extension, not another always-on metric.
They are disabled by default. They neither start/stop containers/models nor
perform inference. Building/testing the feature does not activate it on a
workstation or change existing service configuration.

## Sensitive content and audience

**Logs can contain credentials, prompts/responses, commands, private paths,
addresses and configuration. They are not guaranteed to be redacted.** Rendering
plain text and applying size limits does not make arbitrary logs safe to share.

When enabled, logs use the dashboard's existing admission policy. In explicit
--allow-all-tailnet mode this includes all devices permitted by the existing
tailnet rules, not necessarily only personally owned devices. There is no separate
log-only login or device allowlist. The operator explicitly chose this audience;
reconsider it before enabling logs on a shared tailnet. Never expose them publicly.

Metric and inventory APIs retain their field allowlists. The exception is text
from an explicitly enabled and selected log source, including the currently loaded
model when automatic following has been explicitly enabled. Inspect configuration,
healthcheck output, arbitrary daemon endpoints and upstream event/configuration
APIs still cannot be exposed through the log viewer.

## Sources and synthetic examples

For harmless demonstration, without contacting any real log provider:

```sh
./bin/riglight --demo --logs
```

Demo supplies proxy diagnostics, one model and one container as clearly labeled
synthetic sources. Combine other explicit demo metric flags if desired. Use this
mode for tests, screenshots, examples and bug reproduction, never real log text.

For optional live llama-swap sources, compose the log flags with the dashboard's
existing configuration. Synthetic example:

```sh
./bin/riglight --logs --llama-swap --llama-log-proxy --log-model example-model=example-model
```

--llama-log-proxy offers proxy diagnostics; repeatable --log-model maps a safe
display alias to one explicit model reference (maximum 16 unique selections).
References must be safe single path segments, not slash-containing IDs or reserved
proxy/upstream/http channels. Ambiguous references fail configuration rather than
selecting a broader stream. Unknown/unavailable model loggers never load a model.
Only the fixed GET log-stream routes are used. The fixed /v1/models inventory
collector remains independent. Combined, all-upstream, HTTP access, /running,
configuration, general events, inference and mutable routes stay excluded.

### Follow whichever model is loaded

Instead of individual mappings, explicitly enable the single-current-model source:

```sh
./bin/riglight --logs --llama-swap --llama-log-proxy --log-loaded-model
```

This offers **Currently loaded model**. Start it once to follow the single model
reported LOADED by the existing inventory, switching within the same bounded
connection when that target changes. Old output and the filter are cleared before
new-model lines appear; no mixed-model history is retained. The selected model's
safe inventory identifier is shown beside its output. LOADED is not readiness.

No loaded model means waiting without opening a model log reader. Stale or
incomplete inventory stops reading; multiple reported LOADED models are ambiguous
and none is selected. Unsupported identifiers never fall back to a broader log
channel. Age expires after six seconds even if inventory stops publishing. Fresh
samples for the same target do not reopen the reader or repeat history.

--log-loaded-model requires both --logs and --llama-swap, and cannot be combined
with --log-model. Proxy and Docker selections remain independent. It uses only
existing inventory results, not GPU/process scans, /running, extra discovery,
inference, model controls or a new browser metric connection. Pause retains the
original model label; Resume attaches the current eligible target and clears old
output if it differs. There is still no automatic EOF/error/limit reconnect.
The five-minute/2-MiB/4096-line limits apply across all models in that session;
changing models does not reset them. Start/Resume remains explicit after limits.

For a synthetic demonstration only:

```sh
./bin/riglight --demo --logs --llama-swap --log-loaded-model
```

No live provider is constructed in this demo. See the
[automatic source contract](spec-loaded-logs.md).

For Docker, the separately configured filtering helper also needs --logs.
Synthetic examples, NOT replacement instructions for a running installation:

```sh
./bin/riglight-docker-proxy --logs --listen 127.0.0.1:9123 --socket /run/example/docker.sock --container example-api=example-api
./bin/riglight --logs --docker --docker-proxy-url http://127.0.0.1:9123
```

Only configured helper selections are offered. Browsers cannot select arbitrary
Docker references/IDs, sockets, URLs or API options. The helper validates inspect
identity and required TTY framing internally, then requests only that full ID's
stdout/stderr tail+follow logs. It never forwards raw inspect data. A log session
is pinned to its opening container instance; reopen after a replacement. There
is no automatic discovery or transparent switch to another instance.

Daemon access remains root-equivalent, despite selected GET-only implementation.
Do not grant it to the dashboard. New log connections are independently bounded
from helper metric polling. The helper's loopback Host/Origin checks are browser
request guards, not local authentication: authorized local processes can read
its enabled selected log sources. See [Docker access](docker-access.md).

On systemd, preserve all previously selected options in each single private
RIGLIGHT_ARGS / RIGLIGHT_DOCKER_ARGS assignment; do not replace them with a short
example, source environment files as shell scripts or assume drop-ins append CLI
flags. No service restart/upgrade or permission change is implied. Follow the
separately approved [deployment procedure](deployment.md).

## Using the inspector

The collapsed Logs section does not fetch a catalog or open a log connection.
Open the inspector, choose an offered source, read the warning and explicitly
Start. Catalog refresh before Start/Resume binds the source and DEMO/LIVE mode.

- Pause closes the connection, retaining only the displayed bounded buffer.
  Resume opens a fresh session; upstream history may repeat. No catch-up/replay
  guarantee or automatic reconnect is made.
- Follow-tail controls scrolling, not source activity. Filtering is local text
  matching, not a server query. It cannot be used to request an unselected source.
- Clear discards displayed lines without resetting the current sequence guard.
- Switching sources closes and clears the prior session/filter and requires a new
  Start. Close/unmount disconnects and discards all retained log/filter state.
- Ended/unavailable/limited states are explicit. Quiet output and historical log
  lines do not establish process health, model readiness or collector freshness.
- No persistence, local storage, downloads, export, terminal/HTML/Markdown or
  automatic link rendering. Manual OS copy is still possible; protect copied text.

## Resource limits and failure behavior

Each browser retains at most 200 lines AND 256 KiB of UTF-8 text. Lines are at
most 4096 UTF-8 bytes; truncation is marked. Invalid UTF-8 is replaced and unsafe
control characters removed, not treated as executable output or guaranteed secret
redaction. Exact schemas reject oversized, foreign-source/mode, replayed or
malformed messages instead of displaying them as trustworthy data.

Each process permits two active log sessions independently of metric clients.
Excess opens receive 429 without starting another provider. Each session ends at
five minutes, 2 MiB raw upstream-body input or 4096 emitted lines, whichever comes
first; automatic model changes do not reset those limits. Protocol frames are
capped at 32 KiB. Raw framing, discarded controls and
truncated text count toward input budgets. Catalogs have at most 33 entries and
16 KiB. Fixed read buffers and downstream write deadlines bound slow consumers.

Upstream headers have a ten-second wait. A quiet llama-swap stream may not flush
its headers until output arrives; timeout is shown honestly, not fabricated idle
output or readiness. Reopen manually if needed. Closing or shutdown cancels reads,
closes bodies and releases slots within the existing application stop budget.
Log activity must not renew/stall or restart CPU/GPU/model/container/system metrics.

Unsupported upstream versions, missing model loggers, disabled helper logs,
malformed data, replacement/EOF and transient failures are reported generically.
No raw errors, private URLs, references, inspect fields or upstream response bodies
are placed in diagnostic statuses, journals, telemetry or public reports. Log text
itself is only sent to the explicitly opened, admitted viewer.

See [contract](spec-logs.md), [task plan](../tasks/logs-plan.md) and
[API](api.md). Live deployment/installed-version compatibility and boot behavior
need separately approved verification. Safari/WebKit is not required or claimed.
