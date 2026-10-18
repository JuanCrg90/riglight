# Riglight

**Your AI workstation, at a glance.**

Riglight is a private, read-only dashboard for Linux workstation CPU and RAM
monitoring. The milestone-1 app provides live updates in a browser and an
explicit synthetic demo mode. It binds to loopback by default; it is not a
remote shell or control plane.

## Status and scope

Milestone 1 covers CPU utilization and RAM used/total. GPU, Docker, llama-swap,
public ingress, authentication, persistence, and service installation are not
included. See the [specification](docs/spec.md), [implementation plan](tasks/plan.md),
and [task list](tasks/todo.md) for acceptance criteria and current project state.

## Run

Install the pinned tools with mise, then build the frontend before building the
Go server (which embeds the built assets):

```sh
mise install
pnpm --dir web install --frozen-lockfile --ignore-scripts
pnpm --dir web build
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
```

Run synthetic data without reading host metrics:

```sh
mise exec -- go run ./cmd/riglight --demo
```

By default, the server listens only on `127.0.0.1:9090`. Live mode reads CPU
and RAM metrics only. The optional development mode allows the local Vite
origin `http://127.0.0.1:5173`:

```sh
mise exec -- go run ./cmd/riglight --dev
```

For remote viewing, keep Riglight loopback-bound and use an SSH tunnel from the
client:

```sh
ssh -N -L 9090:127.0.0.1:9090 user@host.example
```

Then open `http://127.0.0.1:9090` locally. Do not expose the listener publicly;
VPN reachability alone is not authorization. The application does not configure
network access or install a service.

## Development checks

Available frontend scripts include `test` (one-shot Vitest), `lint`,
`typecheck`, and `build`. Frontend dependency install scripts are blocked by
default. Follow the [plan](tasks/plan.md) for command ordering and verification;
never claim a check passed unless it was run successfully. Browser `test:e2e`
is planned, not currently listed as a package script.

## Privacy

No telemetry, persistence, prompts, process arguments, or external frontend
assets are part of milestone 1. Public fixtures, screenshots, and logs must be
synthetic. Review files and history for private information before publication;
secret scanners cannot prove arbitrary content is safe. Git commits use the
repository owner's intentionally selected existing identity.

## License and notices

Riglight is licensed under the [MIT License](LICENSE). See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for dependency attribution and
license notices.
