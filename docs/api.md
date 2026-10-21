# API contract

All examples are synthetic. The versioned API is same-origin and read-only.
Responses use JSON with UTF-8 encoding.

## Sample schema

`GET /api/v1/snapshot` and the `snapshot` SSE event carry the same object:

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "cpu": {
    "status": "AVAILABLE",
    "sampledAt": "2026-01-01T00:00:02Z",
    "ageMs": 0,
    "utilizationPercent": 42.5
  },
  "memory": {
    "status": "AVAILABLE",
    "sampledAt": "2026-01-01T00:00:02Z",
    "ageMs": 0,
    "usedBytes": 8589934592,
    "totalBytes": 17179869184
  }
}
```

`schemaVersion` is currently `1`. `mode` is `DEMO` or `LIVE`. Every metric has
one of `NOT_READY`, `AVAILABLE`, or `UNAVAILABLE` status. `AVAILABLE` values
include their timestamp, `ageMs`, and measurements. `ageMs` is a nonnegative
safe integer count of milliseconds since successful collection, calculated for
each response or stream frame. For `NOT_READY` and `UNAVAILABLE`, `sampledAt`,
`ageMs`, and every measurement field are `null`; zero is never substituted for
unavailable data. Timestamps are UTC RFC3339 values for display only. CPU
utilization is a finite number from 0 through 100. Memory values are byte
integers where `0 < usedBytes <= totalBytes` for an available metric.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "cpu": {
    "status": "NOT_READY",
    "sampledAt": null,
    "ageMs": null,
    "utilizationPercent": null
  },
  "memory": {
    "status": "UNAVAILABLE",
    "sampledAt": null,
    "ageMs": null,
    "usedBytes": null,
    "totalBytes": null
  }
}
```

No fields outside this allowlist are serialized.

## Endpoints

| Method | Path | Response |
| --- | --- | --- |
| `GET` | `/healthz` | `{ "status": "OK" }` process health only |
| `GET` | `/api/v1/snapshot` | The current sample schema object |
| `GET` | `/api/v1/events` | Server-Sent Events carrying current sample objects |
| `GET` | `/api/v1/containers` | Optional selected-container snapshot described below |
| `GET` | `/api/v1/system` | Optional swap/root-capacity/selected-uplink snapshot |

API and stream responses are same-origin, have `Cache-Control: no-store`, and
require the configured exact `Host`. A missing `Origin` is permitted for
non-browser clients; a supplied origin must exactly match the configured local
origin. Query parameters and request bodies are rejected. Unsupported methods
return `405` with `Allow: GET`. Errors use:

```json
{
  "error": {
    "code": "METHOD_NOT_ALLOWED",
    "message": "method not allowed"
  }
}
```

The stable error codes are `INVALID_REQUEST`, `FORBIDDEN`, `NOT_FOUND`,
`METHOD_NOT_ALLOWED`, `STREAM_LIMIT`, and `INTERNAL_ERROR`.

## Event stream

The stream sends `event: snapshot` frames with the same JSON object as
`/api/v1/snapshot`, flushes each frame, and advises a 2000 ms reconnect delay:

```text
retry: 2000
event: snapshot
data: {"schemaVersion":1,...}

```

At most eight stream clients are accepted at once; additional clients receive a
`503 STREAM_LIMIT` error. A new connection receives the latest sample, not a
history replay, and a cached sample's `ageMs` continues to increase until a new
successful collection.

## GPU snapshot

GPU collection is explicitly opt-in with `--gpu`. `GET /api/v1/gpus` returns the
GPU snapshot; the existing event stream also multiplexes it as a named
`event: gpus` event. The existing CPU/RAM JSON and `snapshot` events are
unchanged. Values are current readings, not history.

Example payload (synthetic; not a real device):

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "isTruncated": false,
  "devices": [{
    "index": 0,
    "utilization": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"percent":42},
    "memory": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":8589934592,"totalBytes":25769803776},
    "temperature": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"celsius":55},
    "power": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"watts":125.5}
  }]
}
```

Overview states are `DISABLED`, `NOT_READY`, `AVAILABLE`, `UNAVAILABLE`, and
`UNSUPPORTED`. `mode` is `DISABLED` only when overview `status` is `DISABLED`;
otherwise it is `DEMO` or `LIVE`. `AVAILABLE` means inventory was obtained, not
that every sensor is healthy, and has 1–16 reported devices. Fixed generic
reason codes are `DISABLED`, `UNSUPPORTED_PLATFORM`,
`BACKEND_UNAVAILABLE`, `DRIVER_UNAVAILABLE`, `NO_DEVICES`, `QUERY_FAILED`, and
`DEVICE_LIMIT`. Truncation at the 16-device limit sets `isTruncated` and
`DEVICE_LIMIT`. If inventory exceeds 16 devices, exactly 16 are reported and
`isTruncated` is true with `reasonCode` `DEVICE_LIMIT`. Unsupported,
unavailable, not-ready, and disabled overview states have an empty device list.
Per-metric states are `AVAILABLE`, `NOT_READY`, `UNAVAILABLE`, or
`UNSUPPORTED`; non-available measurements, timestamps, and ages are null, never
zero-substituted. Freshness is per sensor and becomes stale after six seconds
without valid sensor data.

Only local numeric indices (0–15) and allowed metrics are exposed. No names,
UUIDs, serials, PCI identifiers, processes, raw driver diagnostics, or history
are included. Memory uses the v2 query when available (used memory includes
driver-reserved memory); v1 fallback retains that driver's semantics.
Temperature is Celsius. Power converts milliwatts to watts and can represent a
one-second average rather than an instantaneous reading.

Native collection requires Linux and cgo; unsupported builds retain CPU/demo
operation. Demo with `--gpu` is wholly synthetic and does not initialize NVML.
Native driver calls cannot safely be canceled; GPU collection is isolated from
CPU collection, and shutdown uses a shared bounded five-second wait rather than
unsafe concurrent library unload. Native initialization is attempted once per
process; if it fails, the generic unsupported/unavailable outcome is retained
and a process restart is required to retry. Successful initialization polls
sensors every two seconds. Riglight does not install or modify drivers or make
other system changes.

## Model inventory

`GET /api/v1/models` and the named `models` event on the same existing SSE
connection return only a bounded allowlist. CPU/RAM/GPU payloads are unchanged.
Collection is opt-in with `--llama-swap`; default and demo never construct the
live HTTP source. All values below are synthetic.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "ageMs": 0,
  "isTruncated": false,
  "models": [
    {"id":"example-chat","state":"LOADED"},
    {"id":"example-embed","state":"UNLOADED"}
  ]
}
```

Modes are DISABLED (only for disabled collection), DEMO or LIVE. Status is
DISABLED, NOT_READY, AVAILABLE, UNAVAILABLE or UNSUPPORTED. AVAILABLE and
NOT_READY have null reasonCode; DISABLED requires DISABLED; UNSUPPORTED requires
API_UNSUPPORTED. UNAVAILABLE reasons are CONNECT_FAILED, TIMEOUT, AUTH_REQUIRED,
QUERY_FAILED or INVALID_RESPONSE. Non-available overviews contain models=[],
ageMs=null and isTruncated=false; sampledAt still timestamps the status update.

AVAILABLE has a nonnegative safe integer ageMs computed at serialization from
retained monotonic completion time. At most 128 distinct safe aliases are
returned in stable order; isTruncated signals overflow and exactly 128 returned.
Empty inventory is valid. IDs are bounded valid Unicode without control/format
characters, absolute/tilde/drive paths, backslashes, parent-directory segments
or URI schemes. Slash-separated org/model aliases are allowed and rendered as
plain escaped text. Models have only id/state; state is LOADED, UNLOADED or UNKNOWN.

States are reported by llama-swap's narrow `GET /v1/models` API. Loaded includes
non-stopped processes and **does not prove readiness**. Missing/unrecognized
upstream state is UNKNOWN; finer starting/stopping/ready states are not inferred.
Other upstream fields, private URLs/diagnostics/commands/configuration, logs,
request content and activity are not exposed or retained in the DTO.

The browser keeps one EventSource for all enabled sources. Inventory freshness
expires after six seconds independently of CPU/GPU events; cached timestamp
replays cannot renew it. No model history, controls or export are provided.
The source uses a configured literal-loopback HTTP/HTTPS base URL, normal TLS
verification, no proxy/redirects, a two-second deadline and one-MiB body limit.
URL configuration is CLI-only and is never accepted from a browser.

## Selected Docker containers

`GET /api/v1/containers` and named `containers` events share the existing SSE
writer/connection/client limit. Existing CPU/GPU/model payloads are unchanged.
The dashboard is opt-in with --docker and an explicit live --docker-proxy-url;
default/demo never construct a live source. The separate filtering helper selects
1–16 configured alias/reference pairs, accesses only fixed inspect/stats GETs,
and returns the same allowlisted shape in LIVE mode. No browser selects targets.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:05Z",
  "mode": "DEMO",
  "status": "AVAILABLE",
  "reasonCode": null,
  "ageMs": 0,
  "containers": [{
    "alias": "demo-service",
    "metadata": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":0,"state":"RUNNING","health":"HEALTHY","restartCount":0},
    "cpu": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":0,"percent":125},
    "memory": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":0,"usedBytes":268435456,"totalBytes":1073741824}
  }]
}
```

Top-level modes, statuses and reason codes match the model-overview families,
without isTruncated. AVAILABLE requires 1–16 unique lexically sorted ASCII aliases
([A-Za-z0-9][A-Za-z0-9_.-]{0,63}) and nonnegative safe ageMs. Other statuses have
null ageMs and an empty array; no silent truncation or all-container discovery.
Metadata statuses are AVAILABLE, NOT_READY, UNAVAILABLE, NOT_FOUND or UNSUPPORTED.
AVAILABLE metadata has independent age/timestamp, state CREATED, RUNNING, PAUSED,
RESTARTING, REMOVING, EXITED, DEAD or UNKNOWN; health STARTING, HEALTHY, UNHEALTHY,
NOT_CONFIGURED or UNKNOWN; nullable safe nonnegative restartCount. Other metadata
statuses have all measurements and age/timestamp null. Sensor statuses are
AVAILABLE, NOT_READY, UNAVAILABLE or UNSUPPORTED; non-available measurements and
age/timestamp are null. Missing/unsupported is never a fabricated zero.

CPU uses successful per-container one-shot deltas: first/reset/recreated/count-
change intervals have no percentage. 100% means one core; 0–409600% is permitted.
RAM subtracts valid inactive-file cache, v1 total_inactive_file or v2 inactive_file,
and reports Docker's limit rather than guaranteeing a configured quota. Byte
counts are safe integers with 0 <= used <= total and total > 0. Coarse Docker
health is not application readiness; restart count is the reported daemon value.

Five-second collection is independent of the other sources. Overview, metadata,
CPU and memory expire independently after 15 seconds. Proxy age plus full request
duration and monotonic elapsed time are retained; fetching a cached response does
not make its old data fresh. Bounded per-field timestamp identities prevent replay
renewal and wall-clock rollback does not order freshness. Stale overview hides
rows; stale metadata/sensors hide their values. No container history/export.

No names/IDs except configured display aliases, images, labels, environment,
commands, mounts, networks, healthcheck output, logs or controls cross the helper
boundary. Inspect is necessarily received transiently and unknown fields discarded;
there is no generic pass-through API. The helper uses privileged Unix-socket access
separately from Riglight and admits trusted local processes to the projected DTO,
not arbitrary browser origins. See [the setup boundary](docker-access.md) and
[the exact specification](spec-docker.md). Building/testing it does not install,
run, grant Docker access or verify compatibility with a real daemon.

## System detail

`GET /api/v1/system` and named `system` events are a fifth source on the existing
SSE connection/writer/client slot. Opt in with --system-details; live mode requires
1–8 exact repeatable --network-interface names. Default/demo never construct live
providers. Only root-filesystem capacity is collected; no parameterized disk paths.

```json
{
  "schemaVersion": 1,
  "sampledAt": "2026-01-01T00:00:02Z",
  "mode": "DEMO",
  "swap": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":1073741824,"totalBytes":4294967296},
  "disk": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":214748364800,"availableBytes":268435456000,"totalBytes":536870912000},
  "network": {"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"rxBytesPerSecond":1048576,"txBytesPerSecond":262144}
}
```

Exact top-level/sensor key sets are shown above: all fields are required, unknown
keys and omitted nulls are invalid. Mode is DISABLED, DEMO or LIVE. DISABLED mode
requires all three sensors DISABLED; enabled modes prohibit DISABLED sensors.
Other sensor statuses are NOT_READY, AVAILABLE, UNAVAILABLE and UNSUPPORTED.
Only AVAILABLE has a UTC timestamp, nonnegative safe integer ageMs and nonnull
measurements; every non-available sensor has timestamp/age/all measurements null.
UNSUPPORTED denotes explicit absent capability, not ordinary errors, missing
interfaces or no configured swap. Top sampledAt records the attempt only.

Swap used/total are safe integer bytes with 0 <= used <= total. A fresh (0,0)
means no configured swap; it is not missing data. Root capacity requires safe
integers, total>0 and used+available<=total. Available is Bavail capacity; reserved
space can account for the difference. Any displayed percentage uses used/total.
No swap paging counters, paths, device/filesystem identities or inodes are exposed.

Network RX/TX are finite fractional bytes/second from actual monotonic elapsed
counter intervals, bounded 0..Number.MAX_SAFE_INTEGER. First interval or individual
counter reset is NOT_READY; missing/duplicate selected counters, failed reads,
overflow or invalid arithmetic cannot produce a partial healthy total. Baselines
recover after reset/failure; valid zero deltas are genuinely idle. Selection names
and cumulative counters stay private, and rates do not certify internet/link health.

Each sensor has independent monotonic age and six-second browser expiry. The
bounded 32-timestamp LRU per sensor prevents retained replay identities from
renewing expiry (including failure/A-B-A/shortened/reconnect cases). An evicted
identity may be treated as unseen; there is no unbounded anti-replay guarantee.
Unseen wall-clock rollback timestamps remain valid. No charts/history/export.

The independent two-second worker uses checked raw Linux swap/root-block
arithmetic and the pinned selected-interface counter reader. A blocked kernel/proc
call can make system data stale, but cannot block other sources/HTTP or extend
shared shutdown; no hard syscall-cancel guarantee. Existing Host/Origin/source
admission, GET/input and no-store/security guards apply unchanged. See
[system setup/meaning](system-detail.md) and [the exact contract](spec-system.md).

## Explicit log inspector

Logs are disabled by default and are NOT a sixth telemetry event or snapshot
cache. --logs gates the catalog and separate manually opened SSE. Existing
Host/Origin/source/interface admission applies to both, so all admitted dashboard
readers can access enabled selected logs. **Raw text may contain secrets, prompts,
paths and configuration; it is not guaranteed redacted.** Metric field-allowlist
claims do not apply to text inside this explicit exception.

GET /api/v1/log-sources returns exactly:

```json
{"schemaVersion":1,"mode":"DEMO","sources":[{"id":"llama.proxy","label":"Proxy diagnostics","kind":"LLAMA_PROXY"},{"id":"model.example-model","label":"example-model","kind":"MODEL"},{"id":"docker.example-api","label":"example-api","kind":"DOCKER"}]}
```

Mode is DISABLED|DEMO|LIVE; DISABLED has no sources. Catalogs are at most 16 KiB,
33 unique sources, 80 ASCII bytes per ID and 128 UTF-8 bytes per label. IDs are
llama.proxy, model.<configured-safe-alias> or docker.<configured-safe-alias>;
kind must match the ID. No URL, socket, model reference or Docker ID is exposed.
The helper catalog is internal loopback-only and has exactly schemaVersion:1,
enabled:boolean, aliases:[configured-safe-alias] (at most 16). Helper --logs is
independent opt-in; unavailable/disabled helpers cannot break other log sources.

GET /api/v1/logs/<id> opens one additional manual EventSource, bounded separately
from the eight metric streams. Unknown/malformed IDs are rejected, never used as
arbitrary upstream paths. Catalog refresh precedes explicit Start/Resume. Reject
non-GET, bodies/transfers, any query (including empty ?), Last-Event-ID and
unexpected encoded paths. No replay IDs, automatic reconnect or persistent logs.
The helper exposes the analogous fixed GET /api/v1/logs/<alias> only for its
configured selections and rejects every Origin header, like its metric API.

Named log and log-status events have exact required key sets:

```json
{"schemaVersion":1,"mode":"DEMO","sourceId":"model.example-model","seq":1,"text":"Synthetic example output","stream":"OUTPUT","truncated":false}
```

```json
{"schemaVersion":1,"mode":"DEMO","sourceId":"model.example-model","state":"STREAMING","reasonCode":null}
```

Every frame binds its DEMO|LIVE mode and source to the selected catalog/session.
Sequence is a positive safe integer, starting at 1 and strictly increasing per
new session. Stream is OUTPUT|STDOUT|STDERR. Frames are at most 32 KiB and text at
most 4096 UTF-8 bytes. Text is escaped plain content, never HTML, hyperlinks or
terminal commands. Invalid UTF-8/control handling is not secret redaction.

States OPENING/STREAMING require null reasonCode. ENDED uses EOF; LIMIT_REACHED
uses SESSION_LIMIT|BYTE_LIMIT|LINE_LIMIT; UNAVAILABLE uses DISABLED|UNSUPPORTED|
UPSTREAM_UNAVAILABLE|INVALID_RESPONSE|READ_FAILED. Terminal states close the
connection. Unknown keys/types/reasons, source/mode mismatches and old sequences
must not be rendered as valid output. Resume may repeat bounded upstream history;
there is no catch-up promise or model/container readiness implication.

Each process has two active log slots. Sessions are limited to five minutes,
2 MiB raw input including framing/discarded data, and 4096 emitted lines. Each
main/helper hop applies raw-body caps. Opening headers wait ten seconds; writes
have one-second deadlines. Disconnect/pause/close/source-switch/shutdown cancel
requests and release slots without blocking independent metric collection. The
browser retains at most 200 lines AND 256 KiB UTF-8 text, clearing all log/filter
state on close/unmount/source change. No download, persistence or export.

Only fixed llama-swap proxy/explicit model log-stream GETs and helper-selected
Docker v1.44 inspect/ID-bound logs are permitted. Combined/all-upstream/HTTP logs,
/running, generic upstream events, discovery, inference and controls stay excluded.
See [operator/privacy guide](log-viewers.md), [source/limit contract](spec-logs.md)
and [tasks](../tasks/logs-plan.md). Implementation does not activate live logs.
