# Security policy

## Scope

Riglight is a loopback-bound, read-only dashboard for CPU and RAM on one Linux
host, with optional NVIDIA GPU monitoring enabled only by `--gpu`. GPU output
is limited to numeric device indices and allowlisted metrics; it does not query
or expose device identifiers, process information, or controls. Riglight does
not provide authentication, public ingress, Docker or llama-swap integrations,
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

Keep the server bound to `127.0.0.1` and use an SSH tunnel for remote browser
access. Do not expose it on a public interface or treat VPN reachability alone
as authorization. GPU collection is disabled by default; pass `--gpu` to opt in
to live NVIDIA metrics. Use `--demo` for screenshots and examples; demo mode is
synthetic and does not read host metrics. `--demo --gpu` produces synthetic GPU
metrics without initializing NVML, while `--demo` alone keeps GPU monitoring
disabled. Live CPU/RAM collection remains independent of the GPU option. For
frontend development, run the backend with `--demo --dev` and run Vite in a
separate terminal on its configured loopback port; `--dev` only permits that
origin and does not start the frontend server.

No telemetry or external frontend assets are intended. Security checks and
package installation may contact public registries or advisory services as
described in the plan. Report only verification that was actually run. Public
fixtures and reports must be synthetic, and files/history should be reviewed
before publication; scanners cannot guarantee that arbitrary data is safe.
