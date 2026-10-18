# Contributing

Thanks for helping improve Riglight. The implemented scope includes read-only
CPU/RAM monitoring and explicitly opt-in, read-only NVIDIA GPU monitoring on
one Linux host. Consult [docs/spec.md](docs/spec.md), [docs/spec-gpu.md](docs/spec-gpu.md),
[tasks/plan.md](tasks/plan.md), [tasks/todo.md](tasks/todo.md),
[tasks/gpu-plan.md](tasks/gpu-plan.md), and [tasks/gpu-todo.md](tasks/gpu-todo.md)
before making changes. Read-only llama-swap inventory is a separately approved
increment; follow [its specification](docs/spec-models.md) and
[task plan](tasks/models-plan.md). Docker, logs, model controls/inference,
deployment, authentication, and data-retention work still require approval.

## Privacy and safety

- Use synthetic data only in fixtures, screenshots, examples, bug reports, and
  public logs. Do not commit hostnames, addresses, usernames, real metrics,
  credentials, process arguments, environment contents, or private paths.
- Do not add telemetry, third-party frontend assets, persistence, public
  ingress, or new integrations to milestone 1.
- Keep the listener loopback-only. Do not change firewall, VPN, service, or
  Docker configuration.
- Dependency install scripts remain blocked unless a specific script is
  inspected and approved. Do not broadly enable lifecycle scripts.
- GPU native code is Linux/cgo-specific; retain and verify the no-cgo and
  unsupported-platform CPU/demo builds. A GPU driver or OS-level installation
  is not required to compile or run unit tests.
- Use synthetic fixtures only; never use real GPU hardware readings in unit or
  browser test fixtures. Live hardware checks are separate, explicit read-only
  verification and must not expose identifiers or readings in public artifacts.
- llama-swap tests must use synthetic local HTTP fixtures, never an operator's
  service, model aliases, metadata or logs. The integration may only issue
  GET `/v1/models`; `/running`, general upstream SSE, inference and control
  endpoints are deliberately excluded. Test URL/proxy/redirect/TLS/response
  bounds, cancellation, projected fields and independent inventory freshness.
- Preserve upstream legal attribution and notices.
- The existing Git author identity is intentionally selected by the repository
  owner; contributions do not anonymize or replace that attribution.

## Toolchain and checks

Use the pinned versions through mise. From the repository root:

```sh
mise install
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web test
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mkdir -p bin
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal web
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
mise exec -- pnpm --dir web test:e2e
```

Build frontend assets before Go tests or builds that embed them. A clean
`gofmt -l` check has empty output. Run only checks applicable to your change,
and report commands, results, and unavailable checks accurately. Browser tests
require the corresponding installed browser and system libraries; distinguish
Chromium/WebKit results and disclose missing platform support.

Before submitting, inspect `git --no-pager diff --check` and the full diff. Never include
real operational data in a test artifact. See [SECURITY.md](SECURITY.md) for
private vulnerability reports and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
for dependency notices.
