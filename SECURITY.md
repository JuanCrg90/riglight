# Security policy

## Scope

Riglight is a loopback-bound, read-only dashboard for CPU and RAM on one Linux
host, with optional NVIDIA GPU monitoring enabled only by `--gpu`. GPU output
is limited to numeric device indices and allowlisted metrics; it does not query
or expose device identifiers, process information, or controls. Optional
llama-swap inventory is enabled separately by `--llama-swap` and exposes only
validated aliases and reported coarse states. No inference, model controls,
logs, commands, prompts/responses, request activity or configuration is read.
Riglight does not provide authentication, public ingress, Docker integration,
persistence, or service installation. Do not assume a private network by itself
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

No telemetry or external frontend assets are intended. Security checks and
package installation may contact public registries or advisory services as
described in the plan. Report only verification that was actually run. Public
fixtures and reports must be synthetic, and files/history should be reviewed
before publication; scanners cannot guarantee that arbitrary data is safe.
