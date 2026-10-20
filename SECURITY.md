# Security policy

## Scope

Riglight is a loopback-bound, read-only dashboard for CPU and RAM on one Linux
host, with optional NVIDIA GPU monitoring enabled only by `--gpu`. GPU output
is limited to numeric device indices and allowlisted metrics; it does not query
or expose device identifiers, process information, or controls. Optional
llama-swap inventory is enabled separately by `--llama-swap` and exposes only
validated aliases and reported coarse states. No inference, model controls,
logs, commands, prompts/responses, request activity or configuration is read.
Optional Docker monitoring requires a separate filtering helper and explicit
container selection. It exposes only configured aliases, coarse state/health,
restart counts and CPU/RAM, not raw Docker responses or a daemon socket.
Riglight does not provide authentication, public ingress, persistence or service
installation. Do not assume a private network by itself
is an authorization boundary.

## Reporting a vulnerability

Please do not report suspected vulnerabilities in public issues with real
host data, credentials, logs, or private paths. Contact the repository owner
through the private security-reporting channel configured by the hosting
platform. If no private channel is available, request one through a non-public
contact route before sharing sensitive details. Include a concise impact and
reproduction using synthetic data where possible; do not attach production
logs, prompts, or configuration.

## Safe operation

Default to `127.0.0.1` with an SSH tunnel for remote browser access. An explicitly
approved [direct tailnet mode](docs/tailnet-access.md) instead requires a selected
IP assigned to an up Linux tailscale0, SO_BINDTODEVICE ingress restriction,
an exact MagicDNS Host/Origin, and explicit admission based on TCP RemoteAddr:
either exact Tailscale client IPs or --allow-all-tailnet. All-device mode admits
Tailscale source addresses through the restricted interface under existing
access rules, including other members' devices if permitted by the tailnet; it
does not establish personal ownership. Empty/invalid client lists or mixed modes
fail closed; forwarded/identity headers never grant access. It does not change
Serve, Funnel, tailnet access rules or firewall settings. HTTP travels inside
the encrypted tailnet; no public HTTPS or application login is claimed. Do not
expose a public/LAN/wildcard listener or treat VPN reachability alone as
authorization. GPU collection is disabled by default; pass `--gpu` to opt in
to live NVIDIA metrics. Use `--demo` for screenshots and examples; demo mode is
synthetic and does not read host metrics. `--demo --gpu` produces synthetic GPU
metrics without initializing NVML, while `--demo` alone keeps GPU monitoring
disabled. Live CPU/RAM collection remains independent of the GPU option. For
frontend development, run the backend with `--demo --dev` and run Vite in a
separate terminal on its configured loopback port; `--dev` only permits that
origin and does not start the frontend server.

llama-swap collection is disabled by default. The operator selects a literal
loopback URL; browsers cannot choose it. Only the fixed GET `/v1/models` path
is used, with bounded response headers/body, a two-second request deadline,
no redirects/environment proxies, and normal TLS verification. `/running` and
the general upstream event stream are deliberately excluded because they expose
command metadata and request activity. Unknown response fields are discarded;
raw responses, errors, URLs and diagnostics are never forwarded. Authentication
requirements are reported without an automatic credential or service change.

`--demo --llama-swap` is wholly synthetic and does not construct the HTTP source.
Actual model aliases are private runtime data: never put them or upstream
payloads in public fixtures, screenshots, logs or reports. Treat alias strings
as untrusted text, not HTML/links/commands; no persistence or export is provided.

Docker collection is disabled by default; live --docker requires an explicit
literal-loopback --docker-proxy-url. Disabled/demo never construct its live
source. A separate riglight-docker-proxy executable is the only component with
separately provisioned Unix-socket access. It pins two GET operations per selected
container, validates reference/ID binding and projects away environment, commands,
mounts, labels, images, identities and healthcheck output. It necessarily receives
inspect responses transiently; it must never expose/store/log their raw contents.
A GET-only raw proxy or read-only socket mount is not an equivalent safeguard.
Do not grant the dashboard Docker-group/socket access or automatically run the
helper as root. No permission changes, daemon calls or helper installation are
implied by building/testing it. See [the boundary](docs/docker-access.md).

The helper is loopback-only and serves cached projected DTOs. Its Host/Origin
checks are browser-request guards, not local caller authentication: trusted
local processes can read selected aliases/metrics. The dashboard's admitted
remote readers can see them too. Only explicit operator configuration selects
containers; browsers cannot choose references, URLs, daemon paths or operations.
Both boundaries enforce deadlines/body/header limits, generic errors, monotonic
sample age and no persistence/export. No logs/events/control endpoints are used.
Live daemon compatibility/deployment requires separate verification and consent.

System detail is separately enabled with --system-details and explicit live
--network-interface selections. It projects only swap bytes, root-filesystem
used/available/total bytes and aggregate RX/TX rates. No interface/device/mount
identifiers, IP/MAC addresses, types, processes or raw counters are exposed.
The root path is fixed and browsers cannot select interfaces/paths. Disabled/demo
never construct live sources. Kernel/proc reads are isolated from other collectors
and HTTP; a blocked syscall can make system data stale but cannot extend the
shared shutdown wait. Byte arithmetic and per-interface reset/delta/elapsed-rate
calculations are checked before serialization. Freshness/replay protection is
bounded per sensor; no unbounded history or anti-replay claim is made. See
[the system-detail boundary](docs/system-detail.md). Implementing/testing it does
not authorize actual interface discovery, live host-data verification or app
restart. These remain later operator decisions.

No telemetry or external frontend assets are intended. Security checks and
package installation may contact public registries or advisory services as
described in the plan. Report only verification that was actually run. Public
fixtures and reports must be synthetic, and files/history should be reviewed
before publication; scanners cannot guarantee that arbitrary data is safe.
