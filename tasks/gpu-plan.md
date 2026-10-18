# Plan: read-only NVIDIA GPU monitoring

Status: completed and reviewed, with WebKit/Safari limits disclosed in
[GPU verification](../docs/verification-gpu.md). Scope was chosen explicitly by
the owner after CPU/RAM accuracy verification. Contract: `../docs/spec-gpu.md`.

## Sequence and ownership

1. Preserve the reviewed CPU one-decimal fix and record the nonintrusive running
   server/reference comparison. Do not restart that server.
2. Commit the GPU spec and bounded task list as the common worktree baseline.
3. Parallel isolated features:
   - Backend: GPT-5.6 Terra high, because native-library lifecycle, concurrency
     and backward-compatible SSE require careful reasoning.
   - Frontend: GPT-6 Luna medium against the fixed synthetic GPU contract.
   - Documentation/notices: GPT-6 Luna low, exclusive documentation files once
     dependency graph and runtime commands are final.
4. Separate GPT reviewer with a fresh context; primary triages every finding.
5. Cherry-pick only reviewed/tested commits into main; run combined verification.
6. Explicit read-only live GPU validation on test-owned loopback ports, with
   no actual readings/identities retained or workloads/host settings changed.

Only the primary coordinates, changes architecture, merges or decides completion.
Workers do not control Herdr or communicate directly with one another.

## Dependency and compatibility decisions

Add only official `github.com/NVIDIA/go-nvml v0.13.4-1` and necessary transitive
modules. Release/health/license checks and relevant pinned sources are described
in the spec. Do not regenerate bindings or run upstream tests that query UUIDs,
process names, driver versions or hardware inventories.

Use native versioned memory and temperature calls first, with explicit legacy
compatibility fallbacks where supported; power is mW converted to W. Preserve
required NVML header disclaimer/US-government notice in documentation and
adapter comments, and exact Apache/other module licenses in distribution files.
Do not paraphrase license terms. Verify bytes/normalized text against upstream.

Keep CPU JSON and named snapshot event unchanged. New gpus named events share
the existing EventSource and slot/write limits. This is an additive transport
extension, not a parallel API version or an extra connection per browser.

GPU is opt-in. A disabled collector constructs no native driver, and demo
constructs only a synthetic source. Native library work belongs to one separate
GPU worker so blocked calls never delay CPU publication. Close only after reads
have finished; bound how long the CLI waits for cleanup, not by racing unload.

Extract a minimal canonical latest-value hub before adding a second consumer;
keep that behavior-preserving refactor separate from the GPU feature commits.
Do not otherwise refactor the working CPU source or frontend layout.

## Verification checkpoints

- Shared hub: existing CPU race/copy/ordering tests unchanged and green.
- GPU source: synthetic tests for each sensor/availability/null/range and native
  adapter return mapping; no-CGO build; no host/device access in unit tests.
- Backend integration: legacy CPU snapshot compatibility, combined SSE initial/
  update frames, eight-client limit, blocked GPU versus CPU, bounded shutdown.
- Frontend: strict GPU contract, per-sensor monotonic freshness, repeated frames,
  unsupported/disabled/partial states, one cleaned-up EventSource, accessible UI.
- Integrated demo: CPU-only and GPU-enabled Chromium browsers, no external
  requests/actual identities, responsive checks, unchanged security guards.
- Live reference: explicit GPU source versus numeric nvidia-smi reference,
  permitting independently documented sampling/rounding/power-average effects.
- Handoff: Go race/vet/format/native and no-CGO builds; frontend tests/lint/types/
  build; audits; exact licenses; Gitleaks public-source/history/artifact scans;
  clean Git; remaining WebKit/Safari limits disclosed rather than hidden.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Native driver blocks/hangs | Independent owner worker; CPU/HTTP continue; bounded CLI cleanup wait; never concurrent unload |
| Unsupported/MIG sensor | Explicit per-metric unsupported/null instead of zero; physical-device-only scope |
| Driver/CGO absent | Unsupported fallback; CPU/demo stay available without GPU initialization |
| Old client breaks | Preserve CPU payload; additive named GPU event only |
| Stale GPU looks healthy while CPU updates | Per-sensor age/expiry independent of CPU events and browser wall time |
| Privacy or native diagnostics leak | Numeric allowlist; no identity/process API calls; generic reasons; no actual logs/fixtures |
| License transcription | Copy upstream verbatim; automate equality; include NVML header notice |
| Existing server interrupted | Never signal/restart it; separate ports and processes for all verification |

No public deployment, remote creation/push, service changes, new JavaScript
libraries, process monitoring, alerts, persistence or other integrations.
