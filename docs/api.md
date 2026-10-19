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
