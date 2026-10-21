# Spec: explicit, ephemeral log viewers

## Approved intent and boundaries

The owner requested log viewers and explicitly chose the current dashboard
admission audience (including all currently admitted tailnet devices), plus
llama-swap proxy diagnostics and selected model-server logs. Docker selection
remains the helper's explicitly configured containers. Combined llama-swap,
all-upstream and HTTP access streams are excluded. Safari/WebKit validation is
not required for this increment; Chromium remains required, not Safari-certified.

Logs can contain credentials, prompts/responses, commands, paths, addresses and
configuration embedded in text. Unlike metric DTOs, log content cannot be
field-allowlisted into guaranteed nonsensitive data. Show an explicit warning;
plain-text display and bounds are protections, NOT reliable secret redaction.
The owner accepted this content risk for the existing audience. Do not fetch real
logs during development, store them in reports/Git, or alter running services.
Implementation approval does not install/restart/activate the live log paths.

## User-visible behavior

One Logs inspector offers an explicitly configured source picker, Start,
Pause/Resume, Clear, Close, local text filter and follow-tail toggle. Starting is
manual, after a warning. Mounting/collapsed UI never opens a source, and source
changes close the old connection, clear its buffer and require explicit Start.
Pause disconnects upstream while retaining the bounded displayed buffer; Resume
opens a fresh session (history may repeat; no replay/catch-up promise). Closing
or unmounting disconnects and clears all log/filter state. Follow-tail controls
scrolling only. No storage, downloads, exports, hyperlinks or terminal rendering.

- At most 200 retained lines AND 256 KiB UTF-8 text per browser inspector.
- Each line is at most 4096 UTF-8 bytes, marked when truncated; invalid UTF-8 is
  replaced and unsafe control characters removed, without claiming redaction.
- Render content as escaped text, never HTML/ANSI/Markdown or executable URLs.
- Show opening, streaming, paused, ended, unavailable and limit states honestly.
  Quiet streams are not proof of availability/readiness; historical lines are
  not current metrics and must not renew other sources' freshness.
- No automatic log reconnect. Native EventSource is explicitly closed on terminal
  status/error; reopening is a deliberate user action, never unbounded replay.
- Existing five metric sources and one telemetry EventSource remain unchanged.

## Configuration and source identities

New optional --logs dashboard flag gates all log routes and source construction.
--llama-log-proxy and repeatable --log-model alias=reference require --logs and
--llama-swap; references are explicit safe single path-segment model IDs, with
reserved proxy/upstream/http names rejected. Model labels use the existing safe
alias grammar; at most 16 unique aliases/references. Unsupported slash-containing
model IDs fail configuration rather than ambiguously addressing an upstream.
No extra model discovery, wakeup, /running, inference, prompts API or control
calls. The separately approved [follow-loaded extension](spec-loaded-logs.md)
adds --log-loaded-model instead of manual mappings, selecting only the fresh,
unique LOADED target from the already running inventory hub.
The configured llama-swap base retains existing canonical loopback URL policy.

Docker logs require dashboard --logs/--docker and separate helper --logs opt-in.
The helper only offers its current explicit selected aliases, never discovery.
Dashboard code never imports the privileged Docker package or receives its socket.
The helper log catalog is a fixed GET returning enabled plus selected aliases;
no IDs, references, configuration or logs are returned by catalog requests.

Browser IDs: llama.proxy, model.<safe-alias>, docker.<safe-alias>. Unknown or
malformed IDs cannot reach a provider. No browser-supplied URL/path/socket,
arbitrary container/model reference or upstream query is accepted. All existing
Host, Origin, source-IP, interface and method guards apply to catalog and streams;
no separate log-only allowlist or authorization bypass is introduced.

## Protocol and bounded transport

- GET /api/v1/log-sources returns schemaVersion:1, mode:DISABLED|DEMO|LIVE, and
  sources:[{id,label,kind}], with fixed kinds LLAMA_PROXY|MODEL|DOCKER. The
  follow-loaded extension adds LOADED_MODEL and its target envelope as specified
  separately; existing fixed-source envelopes and defaults remain unchanged. Demo --logs
  supplies deterministic sources llama.proxy, model.example-model and
  docker.example-api without live clients; unrelated metric flags are not needed
  to read synthetic demo logs. Optional source
  failure cannot break the other catalog entries or existing metrics.
- GET /api/v1/logs/<id> opens a separate manual SSE; never multiplex log text into
  the shared metric hub, caches, journal, telemetry frames or error output.
- Reject non-GET, bodies/transfers, any query including empty ?, unconfigured
  sources, Last-Event-ID and unexpected path encoding. No replay IDs emitted.
- named log events: {schemaVersion:1,mode:DEMO|LIVE,sourceId,seq,text,stream,truncated}; seq starts
  at 1 for each new session and grows; stream is OUTPUT|STDOUT|STDERR.
- named log-status events: {schemaVersion:1,mode:DEMO|LIVE,sourceId,state,reasonCode}; state is
  OPENING|STREAMING|ENDED|UNAVAILABLE|LIMIT_REACHED. reasonCode is null or one of
  DISABLED|UNSUPPORTED|UPSTREAM_UNAVAILABLE|INVALID_RESPONSE|READ_FAILED|EOF|
  SESSION_LIMIT|BYTE_LIMIT|LINE_LIMIT, never a raw error/URL/configuration. Terminal states
  must close the browser connection. Source/session/catalog-mode binding is
  strict; mismatched modes/sources close the viewer without showing text.
  OPENING/STREAMING require null reason; ENDED requires EOF; LIMIT_REACHED
  requires SESSION_LIMIT/BYTE_LIMIT/LINE_LIMIT; UNAVAILABLE uses the other reasons.
  Refresh the catalog before each explicit Start/Resume; never auto-attach a
  different source or mode. Catalog IDs and labels are bounded safe strings:
  IDs at most 80 ASCII bytes, labels at most 128 UTF-8 bytes; catalogs are at most
  16 KiB and 33 unique sources (proxy + 16 models + 16 Docker aliases). Reject
  larger/duplicate catalogs before retaining entries.
- Two active sessions globally per dashboard and separately per helper, plus a
  single browser inspector session; slots acquired before upstream activity,
  released on EOF/error/disconnect/timeout. Overload returns 429 without I/O.
- Each session has a five-minute lifetime, 2 MiB input budget and 4096 emitted
  lines maximum. The input budget counts RAW upstream HTTP response-body bytes
  before parsing/decoding/control removal/truncation, including Docker's eight-byte
  frame headers. Apply the same raw-body ceiling on each main/helper HTTP hop;
  SSE framing/JSON bytes count too. Wire frames are at most 32 KiB; browser preparse caps apply too.
  Sequence numbers are positive safe integers. Fixed bounded
  read buffers, bounded protocol frames and lines, and one-second downstream write
  deadlines prevent slow clients/giant lines from blocking workers or shutdown.
  Input overflow ends explicitly, never silently presents a complete history.
- Opening/header wait at most ten seconds; no replay/retry loops or queued
  unbounded work. A quiet upstream that does not flush headers may time out;
  report this honestly and allow manual reopening.
- Closing, pausing, changing source, shutdown and hard limits cancel requests and
  close underlying bodies. No goroutine-per-read timeout leaks. Logging must not
  extend the existing single five-second shared shutdown budget.

## Fixed upstream operations

llama-swap reference v265, commit a2796fb0306215c6883454a8707dfb22d25c3068:
GET /logs/stream/proxy or GET /logs/stream/<explicit-model-reference> only. Read
bounded initial history then live bytes; no generic /logs, combined stream,
upstream/http channels, arbitrary events, /running, config, inference or controls.
Reject redirects, proxy environment and unexpected content types; unavailable or
unsupported routes become generic states. Opening a model log must not load it.
Official public evidence: internal/server/log.go, server.go and README.md:
https://github.com/mostlygeek/llama-swap/tree/a2796fb0306215c6883454a8707dfb22d25c3068
Installed API compatibility remains a later private verification, not assumed.

Docker helper retains pinned v1.44 operations and adds ONLY selected inspect
identity/TTY projection and GET /v1.44/containers/<validated-full-id>/logs with
fixed stdout=1,stderr=1,follow=1,tail=200,timestamps=0 (no details/config/logfile).
Resolve only an exact configured selection; require a returned ID of exactly
64 lowercase hexadecimal characters (0-9,a-f), reject anything else, and use it
for logs, never browser input. Required TTY boolean is internal-only: framed
stdout/stderr are parsed by Docker's eight-byte multiplex protocol, TTY is plain
output. Malformed headers, unexpected streams, oversized/truncated frames and
missing TTY/identity fail closed; read payload incrementally, never allocate a
frame's untrusted declared size. Preserve split UTF-8/lines and stdout/stderr
separation, mark truncation, and never forward inspect configuration/diagnostics.
A session pins the resolved container instance; replacement requires reopening,
not silently attaching a different instance mid-session. No raw IDs leave helper.
Independent bounded log transport must not consume the metrics polling pool.
Official API reference:
https://github.com/moby/moby/blob/615dfdf67264ed5b08dd5e86657bf0e580731cea/api/swagger.yaml

Helper machine API: GET /api/v1/log-sources and /api/v1/logs/<safe-alias>, both
behind its unchanged exact loopback Host/any-Origin rejection/input guards.
Helper catalog wire schema is exactly {schemaVersion:1,enabled:boolean,aliases:[]}.
Aliases are unique validated configured selections, at most 16, with a 16-KiB
catalog body cap. Use the same bounded log SSE envelope; main validates/rebinds known source IDs,
never blindly proxies bytes/headers. Disabled/demo never construct live clients.
Synthetic demo sources must provide deterministic harmless lines and state cases.

## Implementation and verification

Go stdlib and existing dependencies only. Common bounded protocol/line handling
under internal/logs; optional dashboard CLI/lifecycle wiring, separate server log
handlers, helper-only Docker reader, strict React parser and inspector. Keep
privileged imports out of the main executable's linked dependency graph.

Synthetic tests cover disabled/demo construction, exact upstream method/path/query,
redirects, reserved/path escapes, unknown sources, private-inspect markers,
framed/TTY/split/invalid UTF-8/overlong data, per-stream line assembly, all status
transitions, malformed helper frames, byte/time/line/session limits, slot release,
slow/quiet/blocked sources, cancellation/no leaks, shutdown, and metric independence.
React tests prove strict schema/source/sequence binding, buffer byte+line caps,
manual open, filter/follow/pause/clear/close behavior, no reconnect/persistence,
source switch isolation, escaped hostile text and accessibility.

Run pinned frontend tests/lint/typecheck/build before Go embedding; Go native and
no-CGO tests/builds, race/vet/gofmt, Chromium synthetic E2E and fake Unix Docker
integration, vulnerability/license/dependency and public-history/export/artifact
privacy gates. Use only separate test-owned loopback listeners/fixtures. Never
read live log content for tests, restart services, change permissions or publish.
Independent GPT security/backend and frontend review precedes completion. Report
files/commands/results/limits honestly; Safari is neither gated nor claimed.
