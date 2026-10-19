# Spec: explicit direct tailnet access

Owner approved direct browser access at a workstation MagicDNS hostname/port,
without an SSH tunnel. Actual device addresses/names are private runtime config,
not fixtures/docs/source. No public ingress or Tailscale/firewall policy changes.
Existing Tailscale Serve routes must remain untouched.

## Narrow opt-in

- Default --listen stays loopback-only; preserve all existing flags/behavior.
- New --tailnet enables a separate strict listen policy. Require an explicit
  --listen literal Tailscale IP:port, --host MagicDNS-name.ts.net:matching-port,
  and at least one --allow-client IP (repeatable). Reject these extra flags
  without --tailnet, and reject --dev with --tailnet.
- Reject wildcard, LAN/public/non-Tailscale and DNS listen addresses. Permitted
  address ranges: 100.64.0.0/10 and fd7a:115c:a1e0::/48, normalized/unmapped.
- Before binding, require the selected IP actually belongs to an up Linux
  tailscale0 interface. Range membership alone does not prove Tailscale ownership.
  This mode intentionally does not support userspace networking/custom tun names.
  Additionally bind the listening socket to tailscale0 using Linux
  SO_BINDTODEVICE, so assigned destination IP alone is not the ingress guarantee.
  Fail closed if socket restriction is unavailable; never add privileges or
  silently fall back. Keep lookup/socket controls injectable for unit tests;
  no actual host network configuration in unit tests.
- --host is a bounded lowercase ASCII MagicDNS .ts.net hostname with valid DNS
  labels plus the exact numeric listen port. No URI/userinfo/query/fragment,
  whitespace, trailing dot, IP alias, wildcard, or implicit forwarded Host.
- --allow-client accepts exact IPv4/IPv6 Tailscale IPs only, not CIDRs, DNS names,
  wildcards, loopback, LAN or public addresses. Empty lists fail closed.

## HTTP boundary

- Separate allowed public authority from the socket listen authority. Derive
  the only production Origin as http:// + exact configured --host.
- Add optional Config.AllowedClients []netip.Addr (or equivalently typed field).
  Nil preserves current library behavior; nonnil empty/invalid lists are errors.
  Server configuration independently validates Tailscale range membership, not
  only the CLI. Reject duplicate Host headers at the HTTP parsing boundary.
- When configured, parse the TCP request.RemoteAddr with netip, normalize/unmap
  and require exact client membership before serving anything. Never trust
  X-Forwarded-For, Forwarded, Tailscale identity headers, or browser inputs.
- Preserve exact Host/Origin checks, duplicate/null/foreign Origin rejection,
  security/no-store headers, GET/body/query guards, and bounded single SSE.
  Do not add wildcard CORS, relax public origins, or rewrite Origin blindly.
- Existing loopback mode continues deriving Host/Origin from its listen address.
  Tailnet mode accepts only its explicit hostname, not alternate IP/Host values.
- Source-IP admission relies on Tailscale's authenticated network routing and
  the assigned tailscale0 listener, not arbitrary LAN source addresses. Existing
  tailnet ACLs must also permit the connection; do not edit them automatically.
- HTTP is transported inside the encrypted tailnet. Do not describe it as public
  HTTPS or enable Funnel/Serve, services, firewall changes or external listeners.

## Delivery and verification

Small bounded backend change: CLI parse/network helpers, server Config/security,
focused tests. No dependencies or frontend changes. Keep actual configuration
only in ignored local files/process arguments; synthetic examples use
100.64.0.10, 100.64.0.20 and workstation.example.ts.net.

Tests first: default behavior, all missing flag combinations, bad ranges, invalid
host/port/labels, clients/CIDRs/empty lists, interface up/assigned checks without
host lookup, allowed/denied/malformed RemoteAddr, mapped IPv4, spoofed headers,
Host/Origin rules, GET snapshot/SSE/asset protection, preserved CPU/GPU/models.

Run Go tests/race/vet/gofmt/native/no-CGO builds and unchanged frontend embed
build as necessary; independent GPT security review. No real hardware/service
or operator network calls in unit tests. Primary alone does runtime deployment
using the explicitly supplied private addresses. Do not stop/restart an existing
owner process; if no app is listening, a foreground test instance may be started
for the requested remote test, not installed as a service.

Runtime validation must prove the exact Tailscale bind (not 0.0.0.0/LAN), denial
of other source IPs and spoofed headers, retained Host/Origin guards, no change to
existing Serve configuration, and privacy scans. An actual Mac browser success
requires the owner to open the address; do not claim it was tested remotely
without evidence. Keep simple run/stop/reversal instructions and no auto-push.
