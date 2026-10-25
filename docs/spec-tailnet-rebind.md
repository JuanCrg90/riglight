# Spec: tailnet listener auto-rebind

Owner approved making the tailnet-mode dashboard survive a tailscaled
restart. Observed failure (2026-10-25): tailscaled was restarted and created
a new `tailscale0`; the daemon's `SO_BINDTODEVICE` listener stayed scoped to
the destroyed interface, every connection was refused, and the process
remained alive and silent with no logs.

## Scope

- Tailnet mode (`--tailnet`) only. Loopback, demo, and dev behavior are
  unchanged.
- No admission, Host/Origin, ACL, or security-boundary changes. Rebind uses
  the exact same `SO_BINDTODEVICE` listener path as startup.
- No new runtime dependencies. Interface state uses the stdlib
  `net.InterfaceByName` lookup that the startup check already uses.
- No service, unit, or deployment changes in this increment. Deploying the
  rebuilt binary is a separate approved rollout.

## Behavior

- While serving in tailnet mode, a monitor polls tailnet readiness at a
  2-second interval (same cadence as the metrics publisher).
- Ready means the `tailscale0` interface exists, is up, and holds the
  canonical `--listen` IP, using the existing startup state/lookup helpers.
- When readiness is lost while serving, rebind: close the current HTTP
  server (listener and in-flight connections stop), keep polling until
  ready, create a new listener through the same tailnet listener factory,
  and serve the same handler again. Hubs, workers, publisher, and log
  sources are untouched.
- Rebind attempts continue indefinitely at the polling interval; interface
  churn never exits the process. Interface loss and restore are logged to
  stderr for journald.
- Startup behavior is unchanged: the fail-closed pre-bind check runs before
  the first serve.
- A listener bind failure after readiness is restored is a real error and
  stops the service, matching current fail-closed semantics.
- Browsers use native `EventSource`, which reconnects automatically; no
  frontend changes.
- Exactly one HTTP server serves at any time; the existing shutdown path on
  context cancellation is unchanged.

## Tests

Tests first, with an injected readiness source and fake listeners (no real
host network in unit tests):

- Serving continues while readiness holds.
- Readiness loss closes the current server, stops serving, and waits.
- Restored readiness serves again on a new listener with the same handler.
- Multiple churn cycles create no server/listener leaks; one live server.
- Context cancellation stops the monitor and never starts a new server.
- Readiness errors are treated as not ready (fail-closed) and cannot panic.

## Verification

- Go tests with race detector, vet, gofmt, native and no-CGO builds;
  frontend build unchanged.
- Independent GPT security/concurrency review of the diff.
- Runtime proof is part of the separately approved rollout (owner service
  restart is not part of this increment).
