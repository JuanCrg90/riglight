# Plan: deployment examples, not installation

Status: examples and guide prepared, independently reviewed and verified offline.
See [the evidence](../docs/verification-deployment.md). No services were installed,
enabled or runtime/boot-tested; a separate approved rollout remains necessary.

Follow [the deployment specification](../docs/spec-deployment.md). Owner approved
packaging/docs only; existing dashboard/helper, accounts, groups, permissions,
Docker/Tailscale, services and network rules remain untouched. No application or
dependency changes. Use synthetic values only.

## Ordered tasks

- [x] Freeze preparation scope/sandbox and lifecycle decisions; independent GPT
  review before implementation. Primary: docs/spec-deployment.md and AGENTS.md.
- [x] Unit writer in an isolated GPT worktree adds two system-manager service
  examples and two private environment examples, with distinct pre-provisioned
  identities, conservative loopback/default collectors and no Docker grants.
  Files: deploy/systemd/*.service and *.env.example. Verify static unit invariants.
- [x] Add optional tailnet/GPU/Docker ordering drop-ins, each separate capability
  choice rather than silently enabling features. No dependency-driven host
  service starts, broad permissions or fail-open networking. Verify composition.
- [x] Add stdlib Python tests and offline systemd-analyze verify for examples and
  drop-in combinations without install/start/daemon-reload or manager mutation.
  Use temporary staging and substitute executable paths only; document that
  static verification is not runtime sandbox/boot/identity/driver compatibility.
  Files: deploy/systemd/test_examples.py plus minimal required fixture changes.
- Checkpoint: separate GPT security review and primary verification; writer owns
  only deploy/systemd/**, primary owns architecture/integration and docs.
- [x] Primary prepares docs/deployment.md: prerequisites, staged private config,
  systemd argument semantics, least privilege, optional integrations, startup
  readiness/rate limits, stop/start/enable approval boundaries, port-conflict
  avoidance, verification and rollback. Update README with a clearly scoped link.
- [x] Re-review combined examples/docs; fix actionable findings with one writer.
  Run focused tests, systemd syntax/static checks, docs links/fences/shell syntax,
  diff/secret/history/export/privacy and unchanged-source/dependency gates.
- [x] Record docs/verification-deployment.md and completed scope; clean small
  verified commits, no service installation/enablement, reboot test, publication
  or changes to existing foreground processes. Report all unavailable validation.

## Verification commands (from repo root)

```sh
python3 -m unittest discover -s deploy/systemd -p 'test_*.py' -v
git diff --check
```

Additional exact offline systemd-analyze command and its fixture substitutions
are recorded by the writer's test harness and verification report. Do not execute
operator rollout commands during verification. No full application test rerun is
necessary unless Go/React behavior changes; existing runtime gates stay unchanged.
