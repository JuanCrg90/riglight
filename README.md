# Riglight

**Your AI workstation, at a glance.**

Riglight is a private, read-only dashboard for Linux workstation CPU and RAM
monitoring. The milestone-1 app provides live updates in a browser and an
explicit synthetic demo mode. It binds to loopback by default; it is not a
remote shell or control plane.

## Status and scope

Milestone 1 covers CPU utilization and RAM used/total. Optional, read-only
NVIDIA GPU monitoring is also implemented and must be explicitly enabled with
`--gpu`; Docker, llama-swap, public ingress, authentication, persistence, and
service installation remain out of scope. See the [CPU/RAM specification](docs/spec.md),
[GPU specification](docs/spec-gpu.md), [GPU implementation plan](tasks/gpu-plan.md),
and [GPU task list](tasks/gpu-todo.md) for scope and project state.
[Verification results](docs/verification.md) include passing Chromium coverage;
WebKit/Safari verification remains incomplete.

## Run

Install the pinned tools with mise, then build the frontend before building the
Go server (which embeds the built assets):

```sh
mise install
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web build
mkdir -p bin
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
```

The normal live run reads CPU and RAM metrics only and listens on
`127.0.0.1:9090` by default:

```sh
./bin/riglight
```

Open `http://127.0.0.1:9090` in a browser. For synthetic demo data without
reading host metrics:

```sh
./bin/riglight --demo
```

For frontend development, use two terminals. Start the backend in explicit
synthetic/dev mode:

```sh
mise exec -- go run ./cmd/riglight --demo --dev
```

In the second terminal start Vite, which is configured for loopback port 5173:

```sh
mise exec -- pnpm --dir web exec vite
```

Visit `http://127.0.0.1:5173`. `--dev` permits this local Vite origin; it does
not start the Vite server.

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
`typecheck`, `build`, and `test:e2e` (Playwright). For example, run frontend
checks with `mise exec -- pnpm --dir web test` and browser tests with
`mise exec -- pnpm --dir web test:e2e`. Frontend dependency install scripts are
blocked by default. Follow the [plan](tasks/plan.md) for command ordering and
verification; never claim a check passed unless it was run successfully.
Browser/platform coverage depends on available browsers and system libraries.

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

## Optional GPU monitoring

The approved GPU increment adds opt-in, read-only monitoring of physical NVIDIA
devices. Start with `--gpu` to enable collection; GPU collection is disabled by
default. `--demo --gpu` uses synthetic GPU data without initializing NVML, while
`--demo` alone keeps GPU collection disabled. GPU support is limited to 16
devices; excess inventory is marked truncated. Only numeric indices and
allowlisted measurements are exposed—no hardware identifiers, process details,
or GPU history.

GPU collection requires Linux and cgo. Builds without cgo or on unsupported
platforms retain CPU/demo operation and report GPU support as unsupported. A
working GPU driver is not required to compile. Native driver calls cannot be
safely canceled: a blocked GPU call is isolated from CPU collection, but
shutdown has a shared five-second wait budget and cannot guarantee recovery from
every driver hang. Native initialization is attempted only once per process; if
it fails, Riglight retains a generic unsupported/unavailable result and requires
a restart to retry. After successful initialization, sensors are polled every
two seconds. Riglight does not install, upgrade, or otherwise change drivers or
system configuration.

Start live GPU collection explicitly:

```sh
./bin/riglight --gpu
```

For synthetic GPU readings without initializing NVML:

```sh
./bin/riglight --demo --gpu
```

See [GPU API details](docs/api.md#gpu-snapshot), [security policy](SECURITY.md),
and [third-party notices](THIRD_PARTY_NOTICES.md).
