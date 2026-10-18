# Riglight

**Your AI workstation, at a glance.**

A planned private, read-only web dashboard for Linux AI workstations: system
health, GPU pressure, llama-swap activity, and Docker containers in one view.
Access it from a desktop browser over a trusted private connection.

## Status

Planning stage. There is no runnable application yet. The
[approved milestone-1 specification](docs/spec.md) defines scope, privacy rules,
and acceptance criteria. Review the proposed [implementation plan](tasks/plan.md)
and [task list](tasks/todo.md) before application implementation.

## Privacy first

- No analytics, telemetry, hosted fonts, or third-party frontend assets.
- No prompts, model responses, environment variables, or command-line arguments
  collected by default.
- Live application logs are an explicit opt-in and may contain sensitive data.
- Local settings and runtime data do not belong in source control.
- Public examples, fixtures, screenshots, and bug reports must use synthetic data.
- A private network is not a substitute for least privilege or access controls.

The public repository must not contain real machine identifiers, addresses,
credentials, logs, or personal filesystem paths. Git ignore rules are only one
layer: review files and history before publishing. Secret scanning cannot prove
that arbitrary content contains no personal information.

## Intended stack

Go, React, TypeScript, Vite, and Server-Sent Events. No database for the initial
live-first dashboard. Toolchain versions and dependencies will be pinned during
scaffolding after maintenance and security checks.

## License

[MIT](LICENSE).
