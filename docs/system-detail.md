# Optional system detail

System detail adds swap usage, root-filesystem capacity and aggregate receive/
transmit rates for explicitly configured uplinks. It is disabled by default.
No process information, interface/device identities, addresses, MACs, mount
paths, filesystem types, disk I/O, controls, history or export are displayed.

## Build and synthetic use

Use the pinned toolchain and build the frontend before embedding it:

```sh
mise exec -- pnpm --dir web install --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web build
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
./bin/riglight --demo --gpu --llama-swap --docker --system-details
```

Open `http://127.0.0.1:9090` for an entirely synthetic combined view. Demo system
detail never constructs live providers, reads host/kernel counters or discovers
interfaces. It requires no configured uplinks. Disabled mode also makes no calls.

## Prepare live configuration privately

Live mode requires 1–8 explicit unique interface names. The example below is
synthetic, not an observation about this workstation:

```sh
./bin/riglight --system-details --network-interface example0
```

Repeat --network-interface to aggregate additional deliberately selected uplinks.
Names must be exact bounded Linux interface names accepted by the CLI; no globs,
paths, URLs, whitespace, duplicate names or loopback lo. Interface flags without
--system-details and live enablement without a selection fail closed. The selected
names stay private CLI configuration, not API/stream output. Do not post actual
names or launch arguments in public reports. Browsers cannot choose interfaces.

Only numeric counters are read. There is no automatic route/physical-link,
partition/mount, address, MAC or process discovery. Choose uplinks privately to
avoid counting the same traffic through physical, Docker and VPN adapters. Rates
include the traffic on exactly what was selected; they do not measure internet
speed, prove connectivity or certify link health. A missing/duplicate selected
counter invalidates the aggregate rather than silently undercounting it.

The root filesystem is fixed: no configurable mount list or storage identities.
Live enablement/actual interface selection and an app restart are separate
operator decisions, not performed by implementing or testing this increment.
Preserve existing tailnet/GPU/model/Docker flags if privately enabling the feature.
Systemd users place all options in the single private RIGLIGHT_ARGS assignment;
no extra privilege/device/drop-in is required for these intended read-only paths.
The prepared sandbox still requires real host permission/visibility validation;
see [deployment](deployment.md). Do not install/reconfigure services automatically.

## Read the values correctly

- **Swap:** used and total bytes. Fresh zero total with zero used means no swap
  is configured. Zero usage with positive total is genuine unused swap; neither
  is a failed collector or an unsupported reading. Paging activity is not exposed.
- **Root filesystem:** used, available and total capacity. Available is statfs
  Bavail (space available to an ordinary user); reserved/unavailable blocks can
  make used + available less than total. Percent, if shown, uses used/total, not
  a library's alternate denominator. No mount/device/type label is exposed.
- **Selected uplinks:** RX and TX bytes per second based on actual monotonic
  interval duration, not assumed tick timing. 0 B/s is valid idle traffic. The
  first interval, counter resets and invalid elapsed time have no rate. Failure,
  missing selection or overflow clears the baseline before recovery. An individual
  counter reset cannot be hidden by another interface's increasing total.
- All exported byte values/rates are validated and bounded. Invalid/missing
  measurements are null/unavailable, never zero-substituted. Unsupported means
  an explicit absent capability, not an arbitrary error or no configured swap.

## Freshness and isolation

One independent shared system worker runs every two seconds, not per viewer.
Swap, disk and network have separate successful-completion timestamps and ages.
Browser-monotonic elapsed time expires each after six seconds without fresh data;
other metrics and EventSource reconnects do not renew retained sample identities.
A bounded 32-identity LRU per sensor prevents renewal within that retained window;
evicted identities may be treated as unseen. No unbounded replay/history promise.

Network rate elapsed time and wire age do not rely on synchronized wall clocks.
A raw kernel/proc read can still block despite a context-aware wrapper. One blocked
system worker can make system fields stale, but cannot block CPU/GPU/model/Docker
workers or HTTP, and cannot extend the application's shared shutdown budget. No
unbounded replacement goroutines, hard syscall-cancel guarantee or stress probe.

`GET /api/v1/system` and named `system` frames share the existing protected SSE
connection, writer and eight-client cap. Default loopback and explicit tailnet
Host/Origin/source admission remain unchanged. Permitted readers can see these
numeric values; this feature is not a new authentication or access-policy layer.

## Stop or report a problem

Omit --system-details and its interface options to disable acquisition at the
next explicitly approved restart. Do not stop unrelated owner instances or alter
network policy to troubleshoot unavailable values. Root/statfs/proc visibility and
all measurements should be verified privately after live setup, without induced
load. Use demo data and synthetic provider fixtures in public reports; never
publish actual interface names, counters, metrics, paths, commands or screenshots.

See [the system contract](spec-system.md), [API reference](api.md) and
[implementation tasks](../tasks/system-plan.md). Implementation does not change
existing running instances or establish live accuracy/Safari compatibility.
