# Direct private tailnet access

Riglight defaults to loopback. For an explicitly authorized remote test, direct
HTTP access may be enabled on an assigned Linux Tailscale address, with exact
MagicDNS Host/Origin validation and explicit device admission. Choose a per-client
Tailscale IP allowlist or explicit all-device admission. This is not public
ingress or an application login system; loopback remains the default.

All addresses/names below are synthetic. Substitute your own values privately;
never commit real node names, IPs, client identities or runtime configuration.

```sh
./bin/riglight --tailnet \
  --listen 100.64.0.10:9090 \
  --host workstation.example.ts.net:9090 \
  --allow-client 100.64.0.20 --demo
```

On the permitted client, open `http://workstation.example.ts.net:9090` using
that exact configured hostname, port and scheme. The example is wholly synthetic;
omit --demo for live CPU/RAM and opt into --gpu/--llama-swap separately if wanted.
Do not use an alternate IP or
`localhost`: Host/Origin validation remains enabled. Repeat --allow-client for
additional explicitly authorized devices if required. Changing a client's
Tailscale IP requires updating the private invocation and restarting it.

## All devices in the tailnet

To include current and future devices permitted by your existing tailnet access
rules, replace the --allow-client options with --allow-all-tailnet:

```sh
./bin/riglight --tailnet \
  --listen 100.64.0.10:9090 \
  --host workstation.example.ts.net:9090 \
  --allow-all-tailnet --demo
```

Open the same exact URL on each device while it is connected to Tailscale.
This admits Tailscale source addresses, not arbitrary LAN/public sources. The
listener stays bound to tailscale0; exact Host/Origin checks and stream limits
remain enforced. Do not combine --allow-all-tailnet with --allow-client.

All-device admission is not a personal-device ownership check. Other members'
or shared devices may also connect if your existing tailnet rules permit them.
Use the exact allowlist instead if those devices should not see live metrics
and model aliases. Riglight does not edit the access rules on your behalf.

## Prerequisites and limits

- Kernel Tailscale networking on Linux with an up `tailscale0` interface.
  The selected IP must actually belong to it; the TCP socket is additionally
  bound to this interface with SO_BINDTODEVICE. If the operating system refuses
  that restriction, startup fails without changing privileges or falling back.
  Userspace networking/custom tun names are unsupported in this mode.
- Listen/client addresses must be exact Tailscale IPv4 or IPv6 addresses, not
  wildcard, LAN, public addresses, DNS names or CIDRs. The public authority must
  be a valid lowercase `.ts.net` hostname with the matching explicit port.
- Existing tailnet access rules must permit the client to reach this node/port.
  Riglight never changes these rules, firewall settings, Serve, Funnel or other
  services. Existing Serve routes are independent and must not be overwritten.
- Traffic is HTTP inside the encrypted tailnet, not public HTTPS. Do not enable
  internet forwarding or assume this mode supplies identity/login protection
  outside its assigned Tailscale listener and explicit source-IP admission.
- TCP RemoteAddr is authoritative. Forwarded/identity headers are not trusted.
  Exact-list mode denies other peers even with the expected Host/Origin; all-device
  mode still rejects malformed, loopback, LAN and public source addresses.
- `--tailnet` requires explicit --listen, --host and exactly one admission mode:
  --allow-client options or --allow-all-tailnet. Mixed modes, remote flags without
  --tailnet, or --dev with --tailnet, fail closed.
- Keep the frontend production build current before compiling the Go binary.
  GPU and llama-swap remain separately opt-in; --demo may be used for synthetic
  remote tests without reading hardware or contacting llama-swap.

## Stop and reverse

Run in the foreground; Ctrl-C stops this instance. No system service is
installed. To return to the original default, stop this instance and run
`./bin/riglight` without remote flags: it binds loopback only. Add --gpu and/or
--llama-swap separately only if those live integrations are wanted.
Do not stop unrelated existing instances or reset another Tailscale Serve
configuration. No firewall or tailnet policy rollback is needed because this
mode does not change them.

Server-side probes can confirm the exact bind and denial of unauthorized
sources, but cannot prove a remote Mac browser connected. Ask the owner to open
the URL and report any connection/403 error privately; do not paste real page
contents, model aliases or configuration into public issues.
