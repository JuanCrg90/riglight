# Riglight

**Your AI workstation, at a glance.**

Riglight is a private, read-only dashboard for Linux workstation CPU/RAM,
optional NVIDIA GPU metrics, optional llama-swap inventory, and selected Docker
container monitoring through a separate filtering helper. It provides
live browser updates and an explicit synthetic demo mode. It binds to loopback
by default; it is not a remote shell or control plane.

## Status and scope

Milestone 1 covers CPU utilization and RAM used/total. Optional, read-only
NVIDIA GPU monitoring is also implemented and must be explicitly enabled with
`--gpu`. Read-only llama-swap inventory is enabled separately with
`--llama-swap`; its [specification](docs/spec-models.md) and
[implementation plan](tasks/models-plan.md) define the narrow scope. Optional
Docker monitoring uses `--docker` with an explicit loopback `--docker-proxy-url`;
see [helper setup](docs/docker-access.md), [Docker specification](docs/spec-docker.md)
and [tasks](tasks/docker-plan.md). Only explicitly selected aliases, state/health,
restart counts and CPU/RAM are exposed. The dashboard gets no daemon socket or
raw inspect data; helper deployment/permissions are not automated. Logs, controls,
public ingress, authentication, persistence and service installation remain out
of scope. See the [CPU/RAM specification](docs/spec.md),
[GPU specification](docs/spec-gpu.md), [GPU implementation plan](tasks/gpu-plan.md),
and [GPU task list](tasks/gpu-todo.md) for scope and project state.
[CPU/RAM verification](docs/verification.md) and
[GPU verification](docs/verification-gpu.md) record the earlier milestones.
[Model inventory verification](docs/verification-models.md) records current
combined checks; WebKit/Safari verification remains incomplete.

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

For direct browser access over an existing tailnet, see
[restricted direct tailnet access](docs/tailnet-access.md). That explicit mode
binds only an assigned Tailscale interface address, requires an exact MagicDNS
Host/Origin and requires either configured client IPs or explicit
`--allow-all-tailnet` admission under existing tailnet access rules. Loopback
remains the default;
no public listener, wildcard CORS, Tailscale policy change or service install is
performed.

For a wholly synthetic combined view:

```sh
./bin/riglight --demo --gpu --llama-swap --docker
```

This does not start the filtering helper or contact Docker. Preparing/building the helper is separate from permission to run
it against a real daemon; follow [the access guide](docs/docker-access.md).

## Deployment preparation

Example systemd units and private configuration templates are documented in
[the deployment guide](docs/deployment.md). They use separate unprivileged
identities for the dashboard and Docker helper, conservative loopback defaults,
and explicit optional GPU/tailnet/Docker profiles. Preparing/verifying examples
does not install or enable services, grant access, or interrupt running instances.
A separately approved rollout and real sandbox/boot verification are required.

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

Live/native GPU collection requires Linux and cgo. Builds without cgo or on
unsupported platforms retain CPU/demo operation and report GPU support as
unsupported; synthetic `--demo --gpu` remains available without native GPU
support. A working GPU driver is not required to compile. Native driver calls
cannot be safely canceled: a blocked GPU call is isolated from CPU collection, but
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

## Optional llama-swap inventory

Opt in to the conventional local endpoint:

```sh
./bin/riglight --llama-swap
```

To configure another literal-loopback address, use `--llama-swap-url` together
with `--llama-swap`. HTTP/HTTPS, an explicit port and a root base path are
required; DNS names, credentials, queries, fragments and redirects are rejected.
Environment proxies are not used, and TLS verification is never disabled.
The upstream URL is not sent to the browser. No authentication flow is included;
a protected upstream reports Authentication required without sending credentials.

Riglight reads only `GET /v1/models`, projecting aliases and reported coarse
states. **Loaded does not imply ready**; unsupported/missing state is Unknown,
not Unloaded. Inventory is capped at 128 aliases and independently becomes
stale after six seconds. Requests have a two-second timeout and a one-MiB body
limit. No model loading/unloading, inference, logs, commands, request activity
or configuration is accessed. Defaults remain disabled with no upstream calls.

For synthetic inventory and GPU metrics without contacting llama-swap or NVML:

```sh
./bin/riglight --demo --gpu --llama-swap
```

See [model API details](docs/api.md#model-inventory) and the
[model specification](docs/spec-models.md). All public examples are synthetic;
never share real aliases, inventory or upstream responses in bug reports.
