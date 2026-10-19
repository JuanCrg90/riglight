# Direct tailnet access verification

Delivered an explicit --tailnet mode while preserving default loopback behavior.
The listener requires an assigned/up tailscale0 address, Linux SO_BINDTODEVICE,
an exact configured MagicDNS Host/Origin and exact TCP client-IP admission.
Actual node/client values remain private runtime configuration; all examples
and fixtures in the repository are synthetic.

## Results

- Go tests/race/vet/gofmt and native/no-CGO builds/tests passed. The implementer
  also verified the non-Linux demo build still compiles.
- 118 frontend tests, lint/types and all 17 existing Chromium tests passed.
  No frontend source/dependency changes were needed. Existing WebKit/Safari
  platform limitations are unchanged; no new remote Safari pass is claimed.
- Independent GPT review covered CLI/interface/client validation and the HTTP
  admission boundary. Review requested socket-level interface binding rather
  than relying on assigned destination IP alone; the fix and error/no-fallback
  tests were separately reviewed.
- A test-owned synthetic instance used an assigned Tailscale address and an
  explicitly admitted test source. GET /api/v1/snapshot, /api/v1/gpus and
  /api/v1/models each returned synthetic DEMO data, and one combined SSE supplied
  snapshot/gpus/models events. Wrong Host and foreign/null Origin were rejected.
  Existing security headers remained present. The temporary instance was stopped.
- Separate runtime checks confirmed exact interface-restricted listening and
  denied non-admitted sources, including spoofed forwarding/identity headers.
  Existing Tailscale Serve configuration remained unchanged, verified privately
  by comparing configuration fingerprints. No ACL, firewall, Funnel, service
  or privilege changes were made.
- No owner instance was stopped/restarted. A requested foreground remote-test
  instance can be stopped with Ctrl-C; it is not a system service. Access from
  the real remote browser still requires the owner's confirmation.

Changed areas: CLI network/listener helpers and tests, optional server client
admission/security tests, and targeted operator/security documentation. The
socket restriction fails closed if the OS does not permit it; never silently
fall back or add privileges. Source-IP admission is not an application login
and does not bypass existing tailnet access rules.

Verification used the pinned Go toolchain, go test -race ./..., go vet,
gofmt, -trimpath native/no-CGO builds, frontend test/lint/typecheck and the
Chromium projects. Final gates include unchanged dependency/license checks,
history/export/frontend secret scans, build/source identifier checks and a clean
Git state. No real addresses, names, metrics, inventory, diagnostics or private
configuration belong in the public report. Nothing was pushed or published.

See [the access guide](tailnet-access.md) for synthetic run/stop examples and
[the specification](spec-tailnet.md) for the security boundary.
