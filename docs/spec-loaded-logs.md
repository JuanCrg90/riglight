# Spec: automatically follow the single loaded model

## Owner intent and scope

The owner clarified that the workstation supports one loaded model at a time and
wants to follow that model in turn, not configure individual log-model mappings.
Add an explicit --log-loaded-model option requiring --logs and --llama-swap,
mutually exclusive with repeated --log-model. Proxy and selected Docker log
options, existing admission audience and raw-content warnings remain unchanged.
No real log reads, model actions, inference, new inventory endpoints, live service
activation/configuration, permission changes or publication are authorized.

## User-visible source

A stable source {id:"model.loaded",label:"Currently loaded model",kind:"LOADED_MODEL"}
is offered whenever this option is configured, even while waiting for a model.
The UI prefers it as the initial picker selection when present. The operator
explicitly starts the viewer once; while that single bounded session remains
open, it follows changes without selecting a model or reopening a browser stream.
Pause/Close still disconnect; Resume remains deliberate. No automatic reconnect
on errors/EOF/limits. Existing five-minute/2-MiB/4096-line session caps apply
ACROSS target changes, never reset by swapping models. The same 200-line/256-KiB
client buffer, escaped text, source/mode/sequence validation and no persistence/
export rules apply. Neither loaded state nor output proves model readiness.

On target change, close/cancel the old upstream, discard any queued old lines,
clear the displayed buffer/filter, label the new target and only then show its
output. Retain no mixed-model history. While paused, retained history keeps its
old target label; Resume clears it if the newly announced target differs.

## Backend target selection

Use ONLY the already running llama-swap inventory hub populated by fixed GET
/v1/models; no extra discovery/polling endpoint or second browser metric SSE.
A target is eligible only when inventory mode matches the log mode, is AVAILABLE,
not truncated, and completion age is nonnegative and strictly less than six
seconds by backend monotonic time, with exactly one model reported LOADED.
Ignore UNLOADED/UNKNOWN entries when counting reported LOADED targets, without
assuming any undocumented stronger upstream guarantee.

- Exactly one eligible LOADED model with a safe supported model reference:
  follow that model's fixed /logs/stream/<reference> endpoint.
- Fresh complete inventory with no LOADED target: WAITING; no upstream log I/O.
- More than one LOADED target: AMBIGUOUS; no selection or log I/O.
- Stale, unavailable, pending, truncated or mismatched inventory: UNAVAILABLE;
  immediately cancel any target and wait for a fresh valid inventory.
- Unsupported inventory capability, reserved channel, slash/dot/unsafe model
  reference: UNSUPPORTED; never rewrite/probe a broader route.

Do not infer a target from GPU/processes/commands, /running, access logs, proxy
logs or arbitrary caller input. No fallback to combined/all-upstream channels.
Refresh age via an actual monotonic expiry timer as well as inventory updates;
a hub that stops publishing cannot keep a log stream attached indefinitely.
Repeated fresh samples of the same eligible target must NOT reopen its stream,
clear buffers or repeat history. Root shutdown, expiration and disconnect release
all hub subscriptions, target requests/bodies, bounded queues and timers.

At most one model reader at a time per physical log session. One serialized sink
owns HTTP writes and target/line ordering. Cancel and join the previous reader
before attaching the next; use bounded cancellation-aware handoff, no unbounded
per-read goroutines/queues. Never emit an old target's queued line after a target
transition. All prior admission/write/transport/byte and shutdown bounds remain.
The shared raw upstream budget counts all models' response bytes before parsing;
closing/reopening a target cannot reset it. Existing main line sequence and
lifetime counters remain session-global. A natural EOF/error is terminal, not an
unbounded retry of the same target. Inventory-based target changes are distinct
from network reconnects.

## Additive protocol

Existing fixed-source log and log-status envelopes are unchanged. Catalog kind
LOADED_MODEL is valid only for model.loaded, at most one; it shares the 16-model
source quota with MODEL entries. Fixed MODEL alias loaded remains a fixed source
when the automatic flag is absent; source kind determines behavior, not ID alone.

Automatic sources additionally emit named log-target events, serialized on the
same connection before lines for that target:

```json
{"schemaVersion":1,"mode":"DEMO","sourceId":"model.loaded","revision":1,"state":"FOLLOWING","model":"example-chat"}
```

Required exact keys: schemaVersion, mode, sourceId, revision, state, model.
Mode is DEMO|LIVE and binds the selected catalog/session. Revision starts at 1,
is a strictly increasing positive safe integer within a session. State is
FOLLOWING|WAITING|UNAVAILABLE|AMBIGUOUS|UNSUPPORTED. FOLLOWING requires the safe
selected model reference (maximum 128 UTF-8 bytes); every other state requires
model:null. No URL/configuration/error/process metadata. Frames retain the
32-KiB limit. Emit only when target identity/state changes, not on each sample.

The client accepts these events only for LOADED_MODEL, from its active session,
with exact mode/source/revision/state/null validation. It never displays log
lines before a valid FOLLOWING target, clears old target output on transitions,
and keeps the session line sequence guard across buffer clears. Target changes
must not be confused with a new DEMO/LIVE catalog mode. Retained output keeps
its original target label. Stream OPENING/STREAMING describes the session only;
show the separate waiting/unavailable/ambiguous/unsupported target state honestly.

Demo --demo --logs stays unchanged. --demo --logs --llama-swap
--log-loaded-model replaces the fixed demo model source with the automatic
source and uses only synthetic inventory/model log producers, never live HTTP.
Tests can inject changing inventory/clock/reader factories for deterministic
A -> none -> B, A -> B, A -> stale -> A, and ambiguity scenarios.

## Verification

Test CLI combinations/XOR, strict catalog/target validation, no I/O without a
fresh unique eligible target, exact fixed routes/reserved IDs, expiry without
publications, no same-target reconnection, cancellation/join/no stale queued
lines, aggregate raw-byte/line/lifetime caps, one-reader maximum, quiet/error/EOF,
shutdown and unsubscription. Use synthetic HTTP/clock/hub fixtures only.

Frontend tests cover target modes/revisions/nulls/bindings, line-before-target,
buffer/label/filter clearing, pause/resume session keys and retained labels,
stale callbacks, same-revision replay, waiting/ambiguity without fabricated data,
no browser stream recreation on target changes, and all original bounds.
Add Chromium synthetic target-transition coverage without Safari requirement.
Run full frontend/Go/native/no-CGO/race/vet/build/security/privacy gates and
independent GPT review. Do not read production logs or deploy as part of coding.
