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
`UNSUPPORTED`; `AVAILABLE` means inventory was obtained, not that every sensor
is healthy. Fixed generic reason codes are `DISABLED`, `UNSUPPORTED_PLATFORM`,
`BACKEND_UNAVAILABLE`, `DRIVER_UNAVAILABLE`, `NO_DEVICES`, `QUERY_FAILED`, and
`DEVICE_LIMIT`. Truncation at the 16-device limit sets `isTruncated` and
`DEVICE_LIMIT`. Unsupported/unavailable overview states have an empty device
list. Per-metric states are `AVAILABLE`, `NOT_READY`, `UNAVAILABLE`, or
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
