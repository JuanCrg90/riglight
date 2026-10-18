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
