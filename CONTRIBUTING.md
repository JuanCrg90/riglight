# Contributing

Thanks for helping improve Riglight. Milestone 1 is limited to read-only CPU
and RAM monitoring on one Linux host. Consult [docs/spec.md](docs/spec.md),
[tasks/plan.md](tasks/plan.md), and [tasks/todo.md](tasks/todo.md) before making
changes; later GPU, Docker, llama-swap, deployment, and data-retention work needs
its own approved increment.

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
- Preserve upstream legal attribution and notices.
- The existing Git author identity is intentionally selected by the repository
  owner; contributions do not anonymize or replace that attribution.

## Toolchain and checks

Use the pinned versions through mise. From the repository root:

```sh
mise install
pnpm --dir web install --frozen-lockfile --ignore-scripts
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web typecheck
pnpm --dir web build
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- gofmt -l cmd internal
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
```

Build frontend assets before Go tests or builds that embed them. A clean
`gofmt -l` check has empty output. Run only checks applicable to your change,
and report commands, results, and unavailable checks accurately. The frontend
package currently has no `test:e2e` script; browser testing remains planned.

Before submitting, inspect `git diff --check` and the full diff. Never include
real operational data in a test artifact. See [SECURITY.md](SECURITY.md) for
private vulnerability reports and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
for dependency notices.
