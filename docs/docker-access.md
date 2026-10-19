# Selected Docker monitoring: filtering-helper setup

This increment is opt-in. The dashboard never receives a Docker socket or raw
inspect responses. A separate helper projects state, coarse health, restart
counts and CPU/RAM for explicitly selected containers. It does not expose logs,
controls, images, labels, environment, commands, mounts, network details or IDs.

Implementation and synthetic testing do not grant permission to deploy this
helper. Do not run these live examples until the operator has separately chosen
container references, existing socket access and a deployment arrangement.
Never copy actual names, socket paths, aliases or output into public reports.

## Build and synthetic dashboard

Build frontend assets before the Go executables:

```sh
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web build
mkdir -p bin
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
mise exec -- go build -trimpath -o bin/riglight-docker-proxy ./cmd/riglight-docker-proxy
./bin/riglight --demo --gpu --llama-swap --docker
```

Open `http://127.0.0.1:9090`. Demo mode is wholly synthetic and does not create a
live Docker source or helper. Disabled mode also never contacts the daemon.

## Privilege and privacy boundary

Access to a Docker daemon socket is powerful even if mounted read-only or used
only for GET requests. Provision existing access to the separate helper, not to
the dashboard. Do not add the dashboard user to Docker's group, make sockets
world-readable/writable, expose the daemon over TCP, assume a read-only mount
restricts API methods, or automatically run either executable as root.

The helper must transiently receive inspect data to extract restart counts and
health. That response can contain environment/configuration and healthcheck
output. Unknown fields are discarded, not forwarded, stored, exported or logged.
Only configured aliases and the projected numeric/state DTO reach Riglight.
This is a filtering metrics helper, not a generic HTTP/Docker pass-through proxy.

The helper binds canonical loopback only. Exact Host and rejection of browser
Origin headers prevent browser-request confusion; they do not authenticate local
processes. Trusted local processes can read the projected aliases and metrics.
No multi-user local isolation is supplied. Configure additional host isolation
separately if this trust model is inappropriate.

## Prepare a live invocation (synthetic values only)

This example is for an operator to adapt privately after separate deployment
approval; it is not a request to inspect or change the current daemon:

```sh
./bin/riglight-docker-proxy \
  --listen 127.0.0.1:9123 \
  --socket /run/example/docker.sock \
  --container demo-service=synthetic-service \
  --container demo-worker=synthetic-worker
```

Aliases on the left are the only names displayed. References on the right are
exact Docker names or full lowercase 64-hex IDs, never prefixes, patterns or
URLs. Use 1–16 unique aliases/references; no container discovery is performed.
The helper verifies returned names/IDs before requesting stats. Names can follow
container recreation, but a changed underlying ID resets CPU's first-interval
baseline. Full-ID selections require updating after recreation.

Separately, run the dashboard without socket privileges:

```sh
./bin/riglight --docker --docker-proxy-url http://127.0.0.1:9123
```

The proxy URL must be a literal canonical loopback HTTP/HTTPS origin with no
userinfo, query, fragment or non-root path. No DNS/environment-proxy/redirect
fallback is used; HTTPS retains normal certificate validation. Live mode
requires the explicit URL, and URL-without-enablement is an error. GPU and
llama-swap remain independent opt-ins. Keep the dashboard's existing remote
access flags if privately configured; see [tailnet access](tailnet-access.md).
All readers admitted by that configuration can see selected aliases/metrics.

The helper uses pinned Docker Engine API v1.44. An incompatible API is reported
as unsupported/unavailable rather than triggering broader discovery or arbitrary
version fallback. No installed-engine compatibility is claimed before live
verification. Do not probe the daemon just to fill an example configuration.

## Interpreting measurements

- State is Docker's reported coarse lifecycle, not inferred application readiness.
- Health is a reported healthcheck status; NOT_CONFIGURED and UNKNOWN are honest
  alternatives, not healthy. Healthcheck output is not displayed.
- Restart count is Docker's reported counter, not a count since opening the UI.
- CPU uses successful one-shot counter deltas. The first interval after startup,
  resets or recreation has no percentage. 100% is one CPU core; values above
  100% are valid. Non-running containers have unavailable CPU/RAM, not zeroes.
- RAM subtracts reported inactive-file cache (v1 total_inactive_file or v2
  inactive_file) and shows Docker's reported limit. It is not a promise of a
  configured memory quota. Missing cache support is reported rather than guessed.
- The helper polls every five seconds with a four-second hard budget and bounded
  concurrency. Slow/unstarted operations may be unavailable; starting order
  rotates so later containers are not permanently starved.
- Independent completion ages include proxy transport time. Cached successful
  responses cannot renew underlying data indefinitely. Values expire after
  15 seconds; stale/unknown data must not be treated as healthy.
- `/healthz` means the process is alive, not that the daemon or containers work.

## Stop and reverse

Both examples run in the foreground; Ctrl-C stops the instance you started.
No service, proxy deployment, Docker configuration, permission or tailnet-rule
change is performed by Riglight. To disable collection, omit --docker and its
URL; stop an operator-started helper separately if no longer needed. Do not stop
unrelated dashboard/helper/daemon instances or reset existing network policy.

Never publish live aliases, IDs, inspect bodies, socket paths, screenshots,
metrics or logs when reporting a problem. Use `--demo --docker` and synthetic
fake-daemon fixtures. See [the specification](spec-docker.md) and
[implementation tasks](../tasks/docker-plan.md) for the exact contract and tests.
