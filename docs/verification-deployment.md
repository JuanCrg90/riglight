# Deployment preparation verification

## Scope

Prepared system-manager unit examples for Riglight and its filtering helper,
private environment templates, optional GPU/tailnet/Docker-ordering drop-ins,
and a staged rollout/rollback guide. No application or dependency changes.

No services were installed/enabled/reloaded, accounts or permissions changed,
Docker/Tailscale configured, existing processes restarted, or reboot performed.
A separately approved installation and real sandbox/boot exercise remain required.
Current foreground dashboard/helper are not replaced by preparing these examples.

## Verification performed

- Primary ran python3 -B -m unittest discover -s deploy/systemd -p 'test_*.py'
  -v: **five tests passed**. Original identities, installed executable paths,
  required env files, conservative argument assignments, capability/sandbox/
  lifecycle/dependency settings and optional profile contents are checked.
- systemd-analyze 255 verified staged base, GPU-only, tailnet-only, Docker-only
  and all-options combinations. Positive cases require zero diagnostics; a
  staged invalid drop-in negative control proves drop-ins are actually consumed.
- Each staged ExecStart changes only its not-installed executable path to
  /usr/bin/true, retaining the argument expansion and all other unit contents.
  The verifier never executes it or controls a manager. These are syntax/reference
  checks using the installed analyzer, not installed-binary/permissions/library,
  account, environment-at-exec, cgroup/BPF/device, GPU, runtime or boot validation.
- Separate GPT security review covered identities, daemon privilege separation,
  required environment files, no shell execution, default loopback and collectors,
  optional device/network policies, restart/stop budgets, independence and docs.
  Primary corrected restart-count wording and clarified that process arguments/
  journal metadata can be locally observable, not confidential secret storage.
- Primary followed the inspected systemd 255 manual: unsupported IP-address BPF
  filters may have no effect. They are defense in depth, not a guaranteed startup
  failure or substitute for application interface/Host/Origin/source admission.
- The unit writer's temporary generated-bytecode add/removal pair was excluded
  from main integration. Only the net reviewed text was integrated; no history
  rewriting was used. A local __pycache__ ignore prevents accidental repetition.

Documentation links/fences/shell command syntax and whitespace checks passed.
Go/React behavior, dependency manifests and legal notices were verified unchanged.
Full Git-history and public-export Gitleaks scans passed; public-source/operator-
identifier inspection passed. Main history and export contain no generated
bytecode. No scans substitute for reviewing example/fixture provenance.
No full app or browser rerun was needed for this documentation/example-only change;
the existing Docker increment's test results and WebKit native-library blocker
remain unchanged. No new Safari or live-service success is claimed.

## Files and remaining operator decisions

- deploy/systemd/: two service units, two environment examples, three explicit
  capability/ordering drop-ins, strict unittest verification and bytecode ignore.
- docs/deployment.md: identity/access prerequisites, private configuration,
  systemd argument semantics, readiness/freshness, optional integrations,
  future handoff/enablement and rollback without unrelated service changes.
- README, AGENTS, docs/spec-deployment.md and tasks/deployment-plan.md record
  the limited preparation scope and link the examples/evidence.

Before a real rollout, privately choose/provision identities and existing socket/
device access, review installed binaries/configuration and actual sandbox support,
schedule a foreground-to-systemd handoff, verify health AND enabled freshness,
and decide separately whether to enable boot startup. The example accounts,
addresses and references are synthetic; no operator values belong in public files.
Nothing was pushed or published. See [the guide](deployment.md) and
[specification](spec-deployment.md).
