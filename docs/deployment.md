# Private systemd deployment preparation

These are **examples and a future rollout checklist**, not an installer.
Nothing in this task installs/enables/reloads services, creates accounts, changes
permissions, stops foreground instances, modifies Docker/Tailscale or reboots.
The currently running dashboard and filtering helper stay untouched. Carry out
host changes only after separate operator approval and a scheduled handoff.

## What the examples provide

[deploy/systemd/](../deploy/systemd/) contains two independent system-manager
units, required environment-file examples and optional capability drop-ins.
They target the systemd 255 syntax inspected for this project. Static syntax
verification is not proof of sandbox, boot, driver, account or daemon compatibility.
No Go/React behavior, dependencies or public ingress are added.

- `riglight.service`: dedicated pre-provisioned unprivileged `riglight` account,
  installed `/usr/local/bin/riglight`, required `/etc/riglight/riglight.env`.
- `riglight-docker-proxy.service`: a different pre-provisioned unprivileged
  `riglight-docker-proxy` account, installed filtering-helper executable, required
  `/etc/riglight/docker-proxy.env` and separately approved existing socket access.
- CPU/RAM and loopback are the dashboard default. GPU, llama-swap, Docker and
  tailnet access remain explicit CLI choices, not side effects of installation.
- Missing private environment files prevent startup. The helper also requires
  explicit selected alias/reference pairs and a Unix socket path; there is no
  all-container discovery or ambient daemon endpoint fallback.

## Prerequisites: decide before installing anything

1. An administrator must separately choose/provision the two identities. Do not
   reuse a personal Docker-enabled account for the dashboard. Verify that its
   primary/supplementary groups and socket permissions do NOT grant Docker
   access. Empty `SupplementaryGroups=` cannot erase membership assigned by the
   account database; the examples do not grant a Docker group at all.
2. The helper's daemon access must be an explicit existing-access decision, not
   an automatic group grant. A Docker socket remains root-equivalent capability
   even when mounted read-only or used only for GET. No root service, socket
   chmod, group membership or daemon configuration command is provided here.
3. Review a known release/commit, its dependency notices and tests. Build frontend
   assets before both Go binaries as described in [helper setup](docker-access.md).
   Plan root-owned installed executables, not a service running `go run`, mise,
   writable source files or a personal home-directory launcher. Keep a private
   known-good previous binary/configuration pair for rollback.
4. Review any existing unit/configuration names before staging. Do not overwrite
   unrelated or previously managed units, environment files or binaries. Private
   copies/backup locations must not be committed, archived or posted in issues.
5. Confirm this host's systemd/kernel sandbox capabilities. CPU/RAM needs intended
   read-only `/proc/stat`, `/proc/meminfo` and sysfs visibility; preserving these
   paths does not guarantee readings under every host permission/cgroup setup.
   Check real data freshness during the separately approved rollout.

## Private configuration and systemd argument semantics

Copy/adapt the `.env.example` files privately. Their values are synthetic, not
current workstation settings. Environment files and local backups should be
root-owned mode 0600; system-manager PID 1 reads them before dropping privileges.
The task does not write files under `/etc`, `/usr/local` or a service manager.

`ExecStart` invokes an installed binary directly with `$RIGLIGHT_ARGS` or
`$RIGLIGHT_DOCKER_ARGS`. systemd expands that standalone variable into arguments;
it does not run a shell. Do not use `${RIGLIGHT_ARGS}` (one argument), command
substitution, nested environment references, shell redirections, or assume
`Environment=` can append to an `EnvironmentFile` assignment. Put all flags for
each executable into ONE final assignment. This is configuration, not a safe
place for credentials: process arguments and journal metadata may be observable
by privileged or otherwise authorized local readers. They are not safe secret
storage and must not be copied into public examples or reports.

Synthetic CPU-only default:

```text
RIGLIGHT_ARGS=--listen 127.0.0.1:9090
```

A separately configured filtering helper allows this explicit dashboard option:

```text
RIGLIGHT_ARGS=--listen 127.0.0.1:9090 --docker --docker-proxy-url http://127.0.0.1:9123
```

Use private selected aliases/references in the helper file, not a browser input.
Only the helper gets daemon access. Its loopback API trusts local processes for
projected aliases/metrics; Host/Origin guards are not local authentication. See
[the helper boundary](docker-access.md) before making that access decision.

## Optional capabilities and composition

Install only reviewed, selected drop-ins from `deploy/systemd/riglight.service.d/`:
`10-gpu.conf`, `20-tailnet.conf` and `30-docker-proxy.conf`. Their `.conf` names
belong inside the private system-manager `riglight.service.d` directory. A drop-in does not append CLI flags by
itself: compose the corresponding options in the one private argument assignment.

### Tailnet access

Default IP filtering permits localhost only. Add the tailnet drop-in as well as
explicit private CLI `--tailnet --listen ... --host ...` and exactly one admission
mode. The tailnet profile also adds AF_NETLINK to the inherited address-family
allowlist: Go uses NETLINK_ROUTE for interface/address validation before binding.
Blocking it prevents startup even when the configured address is valid. This adds
no capabilities or network configuration changes; keep the base restrictions,
exact application guards and empty capability sets.

Synthetic example (never replace public examples with actual values):

```text
RIGLIGHT_ARGS=--tailnet --listen 100.64.0.10:9090 --host workstation.example.ts.net:9090 --allow-client 100.64.0.20
```

For an explicitly authorized whole-tailnet policy, use --allow-all-tailnet instead
of --allow-client. Combine wanted GPU/model/Docker flags in this same assignment.
The application still requires an up assigned tailscale0 IP and SO_BINDTODEVICE,
exact Host/Origin and TCP source admission. Never add root/CAP_NET_RAW, wildcard
binds or firewall/VPN changes if startup fails. The unit's range filter is not
personal-device ownership or application login. Existing tailnet rules apply.

`IPAddressDeny/Allow` is defense in depth. systemd 255 documents that filtering
may have no effect when cgroup/eBPF support is unavailable; do not assume it
works or rely on it instead of application guards. Other sandbox/platform
incompatibilities can prevent service startup. Review them explicitly, never
silently strip controls to obtain a running service.

### GPU

The base private device namespace intentionally blocks GPU access. Opt-in GPU
requires --gpu, the GPU drop-in and a private review of exact device nodes,
ordinary device permissions and driver/library access. The example turns off
PrivateDevices but uses a closed device policy permitting only standard
/dev/nvidiactl and /dev/nvidia0 nodes. It is not an all-GPU/MIG compatibility
promise or permission grant. Adjust only reviewed required nodes after approval;
never allow all devices or add root/CAP_SYS_ADMIN as a workaround. Real NVML
operation in this sandbox is unverified. Unsupported readings must remain honest.

### llama-swap and Docker

llama-swap remains an explicit --llama-swap flag and optional canonical loopback
URL. Its process liveness is not model readiness; do not tie the dashboard's
lifecycle to the upstream application or change its service.

The Docker drop-in may Want/After the separately installed helper. It deliberately
does not Require, BindTo or PartOf it: helper failure must not stop CPU/GPU/model
views. Default units remain independent. Do not make the helper start/restart the
Docker daemon or hardcode a host-specific Docker unit name (rootless/Snap/etc.).
An existing API/socket and selected-container configuration are prerequisites.

## Startup, readiness and restart behavior

Ordering is not readiness: network.target and After do not prove addresses,
Tailscale authentication, daemon availability, helper data or GPUs are usable.
The examples do not start Docker or Tailscale. If an optional After ordering is
adapted to an existing Tailscale service, it still does not prove tailscale0 has
its assigned address. Application validation fails closed if it does not.

Type=simple tracks the process, not collector readiness. Restart=on-failure and
RestartSec=5s recover crashes, not unavailable collectors. The example permits
three starts within 60 seconds; a further attempt is rate-limited. After fixing
prerequisites an
operator may deliberately retry/reset the failed unit. Do not assume it retries
forever during a long network boot or infer success from an active status.

Both processes handle SIGTERM. Application shutdown has one five-second budget;
the units allow ten seconds before control-group cleanup. The dashboard and helper
are separate: stopping one does not imply the other stopped. No writeable app
state, telemetry or log export is required. Generic stderr may enter the local
journal; do not publish unit output, resolved arguments or environment contents.

## Staged rollout: future, separately approved operations

1. Validate the repository examples without installation:

   ```sh
   python3 -m unittest discover -s deploy/systemd -p 'test_*.py' -v
   git diff --check
   ```

   The offline syntax harness uses temporary copies and substitutes only the
   not-installed executable paths with an existing harmless executable for
   systemd-analyze verify. Nothing is executed or installed. Original paths,
   directives, flags and env files are checked structurally. This does NOT
   validate installed permissions, NSS identities, libraries, BPF, GPU, socket
   access, real environment parsing at execution, boot or restart behavior.
2. Separately approve/provision identities/access and stage private copies of
   reviewed binaries, environment files, chosen drop-ins and units. Verify the
   private staged/installed originals with systemd-analyze verify as well; do
   not mistake the offline fixture substitution for validation of the installation.
3. Arrange explicit handoff approval. The existing foreground dashboard/helper
   already own their endpoints; do not start competing units or kill by broad
   process/port patterns. Gracefully stop only the identified owned instances
   at handoff time, then confirm ports are free. Keep rollback commands privately.
4. Only after installation and approval, daemon-reload/start the selected owned
   units. Check process health AND all enabled data freshness, snapshot/SSE,
   exact tailnet/interface and helper-loopback binding, source admission, Host/
   Origin denial and other already running views. Do not preserve real readings,
   aliases or screenshots as public evidence. A healthy /healthz alone is not
   acceptance. Test an admitted remote browser without recording its private DOM.
5. Decide separately whether to enable on boot. A boot/restart exercise is a
   scheduled later operation, not something this preparation performed. Do not
   reboot a workstation or restart Docker/Tailscale to test these examples.

## Rollback checklist

After a separately approved rollout, roll back only these owned changes:

- Stop newly started units and disable only ones newly enabled by that rollout;
  never disable an unrelated/preexisting service or follow broad kill patterns.
- Restore the previous reviewed binary/configuration pair from private backup.
  Preserve unrelated units/files; do not destructively reset the repository.
- If unit/drop-in files changed, a separately approved daemon-reload is needed.
  Do not reload managers, change permissions or delete files during this task.
- After ports are free, restore the earlier foreground invocation privately or
  start the previous approved units. Verify health, enabled collector freshness,
  SSE and the original private access boundary. Treat absent/stale values honestly.
- Do not revert Docker containers, daemon state, Tailscale Serve/ACLs or firewall
  policy: deployment packaging does not change them. Report failures privately.

See [the deployment spec](spec-deployment.md) and
[task plan](../tasks/deployment-plan.md) for the preparation boundary and sources.
