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
