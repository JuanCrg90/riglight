# Riglight development

## Scope and coordination

- Follow `docs/spec.md`, `tasks/plan.md`, and `tasks/todo.md`.
- CPU/RAM and optional NVIDIA GPU monitoring are implemented and reviewed.
  GPU remains opt-in with --gpu; no identity/process APIs or GPU controls.
- The owner approved read-only llama-swap inventory/coarse states only; follow
  `docs/spec-models.md` and `tasks/models-plan.md`. Default disabled; no auth flow,
  logs, inference, commands/configuration, request activity or model controls.
  Use only the fixed GET /v1/models upstream API; never /running or general SSE.
- Docker, public ingress, authentication, persistence, logs, and service
  installation remain separate unapproved increments.
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
- Bind loopback only; no firewall, VPN, system-service, or Docker changes.
- No telemetry, third-party frontend assets, or runtime analytics.
- Dependency scripts stay blocked unless individually inspected and approved.
- Preserve unrelated changes. Use Trash for deletions; no destructive Git resets.

## Verification

- Use the pinned toolchain: `mise exec -- <command>`.
- Build frontend assets before Go builds/tests that embed them.
- Run relevant tests, race checks, vet/format, frontend lint/typecheck, and builds.
- Final checks include demo-mode Chromium/WebKit browser tests, dependency audits,
  secret scans, artifact privacy inspection, and a separate GPT code review.
- Do not claim unavailable checks passed. Never bypass hooks or failed tests.
- Report changed files, commit IDs, commands/results, assumptions, and blockers.
