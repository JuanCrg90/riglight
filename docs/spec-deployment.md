# Spec: private deployment packaging

The owner approved preparation of systemd examples and deployment/rollback
instructions after successful private dashboard and Docker-helper use. This is
NOT permission to install/enable/reload services, create accounts/groups, change
permissions, restart existing processes, alter Docker/Tailscale/network policy,
or publish. Current foreground dashboard/helper must remain untouched.

## Deliverables and safe defaults

- Two example system-manager units: riglight.service and
  riglight-docker-proxy.service, under deploy/systemd/. Target Linux/systemd 255
  for syntax verification; no claim of boot or sandbox runtime validation.
- Distinct pre-provisioned unprivileged accounts riglight and
  riglight-docker-proxy. Never root, DynamicUser, personal accounts, Docker
  group grants, SupplementaryGroups=docker or automatic identity provisioning.
  Dashboard identity must NOT already have daemon/socket access; helper identity
  needs separately approved existing access. Unit hardening does not turn daemon
  access into a low-privilege capability. No account/install helper scripts.
- Installed executable paths /usr/local/bin/riglight and
  /usr/local/bin/riglight-docker-proxy; root-owned/release-managed binaries are
  prerequisites, not written by this task. No source-tree/mise/go-run wrappers.
- Required EnvironmentFile entries /etc/riglight/riglight.env and
  /etc/riglight/docker-proxy.env. Synthetic .env.example files only. ExecStart
  uses $RIGLIGHT_ARGS / $RIGLIGHT_DOCKER_ARGS as systemd word-split arguments,
  not shell execution; no nested variable/command substitution. All flags for
  each process belong in ONE assignment, not attempted concatenation in drop-ins.
- Default dashboard CPU/RAM on 127.0.0.1:9090, no optional collectors. Helper
  loopback 127.0.0.1:9123 with explicit synthetic selection/socket placeholder.
  Existing runtime CLI guards remain authoritative. No copied actual addresses,
  usernames, aliases, socket configuration, selected containers or launch scripts.
- Units are independent by default. Optional Docker dashboard ordering/drop-in
  may Want/After helper, never Require/BindsTo/PartOf (helper failures must not
  stop independent host/GPU/models). No automatic Docker/Tailscale starts or
  restarts. Optional After=tailscaled.service ordering is not a readiness proof.

## Sandboxing and lifecycle

Use modest sourced controls rather than unverified syscall filters: Type=simple,
Restart=on-failure, RestartSec=5s, bounded StartLimitIntervalSec/StartLimitBurst,
KillSignal=SIGTERM, KillMode=control-group, TimeoutStopSec=10s (application total
shutdown budget is five seconds), UMask=0077, NoNewPrivileges=yes, empty
CapabilityBoundingSet/AmbientCapabilities, ProtectSystem=strict, ProtectHome=yes,
PrivateTmp=yes, PrivateDevices=yes, ProtectKernelTunables/Modules/ControlGroups,
RestrictSUIDSGID/Realtime/Namespaces and LockPersonality. Keep host /proc/stat,
/proc/meminfo and ordinary read-only sysfs visible: no PrivateNetwork, ProcSubset,
RootDirectory/chroot, /proc masking or hide-host-metrics assertions. No persistence
or writable application state directories are needed. Generic stderr may go to
journal; do not claim systemd metadata/arguments are anonymous or publish logs.

Restrict address families to AF_UNIX/AF_INET/AF_INET6, default IPAddressDeny=any
and IPAddressAllow=localhost. Optional tailnet drop-in adds only the conventional
Tailscale ranges 100.64.0.0/10 and fd7a:115c:a1e0::/48. The tailnet profile also
adds AF_NETLINK to the inherited family allowlist for Go's NETLINK_ROUTE interface
and address validation. This grants no capabilities; the base unit stays unchanged.
Requires explicit private
CLI tailnet flags + up assigned tailscale0 + application SO_BINDTODEVICE and exact
Host/Origin/source admission. No added capabilities if binding fails; fail closed.
Systemd IP filtering is defense in depth, may be unavailable without cgroup/BPF
support, and never replaces application safeguards. Do not change firewall/ACLs.

Optional GPU drop-in must explicitly undo PrivateDevices and use DevicePolicy=
closed with only synthetic standard /dev/nvidiactl and /dev/nvidia0 allowances.
Document that exact nodes/group/device permissions/library access need private
operator review and real sandbox testing; no automatic device discovery, grants,
driver/service changes or claim NVML works inside the example sandbox. Do not
add all devices, CAP_SYS_ADMIN, CAP_NET_RAW or root to resolve a failure.

## Operator guide

Prepare docs/deployment.md covering prerequisites, reviewed release directories/
binaries, private 0600 environment configuration and root-owned units, optional
GPU/model/Docker/tailnet composition, existing-access checks, startup ordering
versus readiness and health versus freshness. Show a staged future rollout only:
verify examples, provision separately approved identities/access, stage/verify
private copies, explicitly schedule foreground-to-systemd handoff to avoid port
conflicts, start only after approval, inspect health + all enabled data freshness,
then enable on boot only after a separate decision. Do not execute these steps.

Describe restart rate limiting/recovery when interfaces/dependencies are late:
network.target/After do not establish address or service readiness; existing
collectors may report unavailable while the process stays healthy. Do not add
RequiresMountsFor, broad dependency restarts or unbounded retries to force it.
Rollback: stop/disable only newly installed owned units after approval, restore
previous reviewed binary/config versions, daemon-reload only if unit files
changed, and restore the earlier foreground invocation privately after ports
are free. Never revert Docker/Tailscale state or stop unrelated owner services.
Retain private backups locally; do not publish systemd output/process args.

## Verification and ownership

No Go/React behavior or dependencies change. Add stdlib Python unittest checks
for exact service identities/binary/envfile paths, safe defaults/drop-in scope,
capabilities/no daemon grants, lifecycle/dependency semantics and absence of
install/start/exec-shell scripts. Validate syntax with systemd-analyze verify
without installation/manager mutations. For offline verification, stage units in
a temporary directory and substitute ONLY the not-installed executable paths
with an existing harmless executable for verify (never execute it); report this
limitation explicitly. Inspect originals structurally and flags against the CLI.
Do not ignore unknown-directive/errors, or call systemctl/systemd-run/enable,
useradd, chmod/chown of host files, apt or any live service controls in tests.
Missing verification tools are explicit blockers, not silently skipped tests.

Separate GPT worker implements only deploy/systemd/** examples/tests in an
isolated worktree; primary owns docs/tasks/AGENTS/README and integration. Separate
GPT security review plus primary triage, focused tests/unit verification, docs
links/fences/shell checks and history/export privacy scans. Existing running
app/helper and private config are not touched, and no sandbox/boot test is claimed.

## Official sources

Semantics checked against the installed systemd 255 manual pages staged privately
for research; no upstream manual text copied into this repository:

- https://www.freedesktop.org/software/systemd/man/255/systemd.exec.html
- https://www.freedesktop.org/software/systemd/man/255/systemd.service.html
- https://www.freedesktop.org/software/systemd/man/255/systemd.unit.html
- https://www.freedesktop.org/software/systemd/man/255/systemd.resource-control.html
- https://www.freedesktop.org/software/systemd/man/255/systemd-analyze.html

See tasks/deployment-plan.md for bounded implementation and evidence gates.
