# Riglight development

## Scope and coordination

- Follow `docs/spec.md`, `tasks/plan.md`, and `tasks/todo.md`.
- CPU/RAM and optional NVIDIA GPU monitoring are implemented and reviewed.
  GPU remains opt-in with --gpu; no identity/process APIs or GPU controls.
- Owner approved optional system detail: swap, root-filesystem capacity only,
  and aggregate RX/TX for explicitly configured uplinks. Follow docs/spec-system.md
  and tasks/system-plan.md. Implement/test only; no actual interface selection/
  discovery, host-data verification or running-app changes without later approval.
- The owner approved read-only llama-swap inventory/coarse states only; follow
  `docs/spec-models.md` and `tasks/models-plan.md`. Default disabled; no auth flow,
  logs, inference, commands/configuration, request activity or model controls.
  Use only the fixed GET /v1/models upstream API; never /running or general SSE.
- The owner approved selected-container Docker monitoring through a filtering
  helper; follow docs/spec-docker.md and tasks/docker-plan.md. Live deployment
  was separately approved and verified with existing access and private selection.
  Do not change its running helper/dashboard or daemon permissions/configuration
  without a new explicit request.
- Deployment examples/documentation were followed by a separately approved
  private installation with dedicated identities and helper-only new Docker access.
  Both services are verified active/enabled; actual boot remains untested.
  Do not change their units/configuration, access, permissions or running state
  without a new explicit rollout request. No reboot is authorized.
- Owner approved implementing optional log viewers; follow docs/spec-logs.md and
  tasks/logs-plan.md. Audience matches current dashboard admission. Sources are
  selected Docker containers, explicit model-server logs and llama-swap proxy
  diagnostics only; no combined/all-upstream/HTTP-access channels. Raw log text
  can contain secrets/prompts/configuration; warn, bound and render as plain text,
  never promise reliable redaction. No persistence/export, real log verification
  or live activation/service changes are authorized by implementation approval.
- Public ingress, authentication, persistence and controls remain separate
  unapproved increments.
- Do not stop/restart the owner's running server. Verify new builds on separate
  test-owned loopback ports; do not inject synthetic stress/benchmark workloads.
  Routine project builds/tests and approved dependency downloads remain allowed.
- Use GPT models through `openai-codex` only for project development and review.
  Do not use local, OpenCode, Gemini, or other non-GPT workers for this project.
- The primary session coordinates. Assigned workers implement only their bounded
  scope; they do not delegate, control other agents, or broaden permissions.
- Parallel feature work uses isolated Git worktrees and exclusive file ownership.
  Do not merge another worker's branch or edit another worktree without assignment.
- Keep commits small and verified. No pushing or remote creation without consent.

## Privacy and safety

- Only synthetic fixtures, screenshots, and examples belong in public files.
- Never commit machine identifiers, addresses, local paths, credentials, real
  metrics/logs, prompts/responses, process arguments, or environment contents.
- Git author identity is the owner's explicitly selected existing identity;
  do not change it or imply that Git authorship has been anonymized.
- Default bind stays loopback-only. Owner approved explicit direct tailnet access
  in `docs/spec-tailnet.md`: assigned tailscale0 IP only, exact MagicDNS Host/
  Origin and explicit client-IP allowlist or explicit --allow-all-tailnet mode.
  The owner approved all-device access subject to existing tailnet rules.
  Actual addresses stay private.
  No wildcard/public/LAN binding, firewall/VPN policy, service, or Docker changes.
- No telemetry, third-party frontend assets, or runtime analytics.
- Dependency scripts stay blocked unless individually inspected and approved.
- Preserve unrelated changes. Use Trash for deletions; no destructive Git resets.

## Verification

- Use the pinned toolchain: `mise exec -- <command>`.
- Build frontend assets before Go builds/tests that embed them.
- Run relevant tests, race checks, vet/format, frontend lint/typecheck, and builds.
- Final checks include demo-mode Chromium browser tests, dependency audits,
  secret scans, artifact privacy inspection, and a separate GPT code review.
  Owner explicitly waived Safari/WebKit validation for the log increment; do not
  claim compatibility or silently remove existing optional test projects.
- Do not claim unavailable checks passed. Never bypass hooks or failed tests.
- Report changed files, commit IDs, commands/results, assumptions, and blockers.
