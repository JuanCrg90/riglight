# Tasks: tailnet listener auto-rebind

Follows `docs/spec-tailnet-rebind.md`. Bounded increment; small verified
commits; tests first; no commits by workers (coordinator commits).

## 1. Generic rebind serve loop

- [x] Add `internal/relisten` package: a serve loop that owns the
  http.Server/listener lifecycle and rebinds when an injected readiness
  source reports loss. Readiness nil (or non-tailnet) preserves the current
  single-serve behavior with no polling goroutine.
- [x] Fail-closed: readiness error counts as not ready; bind failure after
  restored readiness returns an error; context cancellation closes the
  current server and returns without starting a new one.
- Acceptance: fake-listener/injected-readiness tests pass under `-race`
  covering all cases listed in the spec's Tests section.
- Verify: `mise exec -- go test -race ./internal/relisten`,
  `mise exec -- go vet ./internal/relisten`.
- Files: internal/relisten/relisten.go, internal/relisten/relisten_test.go.
- Dependencies: none. Scope: medium.

## 2. Wire tailnet readiness into the dashboard

- [x] In `cmd/riglight` tailnet mode, build the readiness source from the
  existing injected `lookup` + `tailnetInterfaceHasAddress` and pass it with
  the existing tailnet listener factory into the new serve loop; keep the
  startup pre-bind check, publisher ticker, and shutdown path unchanged.
  Non-tailnet mode keeps the plain serve path.
- [x] Log readiness loss and restore to stderr (bounded messages, no
  secrets/addresses beyond the configured listen IP already present in
  process arguments).
- Acceptance: existing tailnet/loopback CLI tests pass; new wiring test
  proves tailnet mode rebinds and loopback mode does not poll.
- Verify: `mise exec -- go test -race ./cmd/...`, `mise exec -- go vet ./...`.
- Files: cmd/riglight/main.go (wiring), cmd/riglight wiring test, plus any
  small helpers needed in cmd/riglight.
- Dependencies: 1. Scope: medium.

## 3. Full verification and independent review

- [x] Full Go suite with race detector, vet, gofmt, native and no-CGO
  builds; frontend build unchanged.
- [x] Independent GPT review (separate worker) of the diff; triage and fix
  findings; re-verify.
- Verify: suite commands above plus
  `mise exec -- go build -tags noCGO ./...` (or the project's established
  no-CGO build flag) and a final `git diff` privacy scan.
- Files: none new (fixes only if review requires).
- Dependencies: 2. Scope: small.

## Out of scope

- systemd units/services changes; binary deployment or owner service
  restart (separate rollout request); frontend changes; netlink
  subscription (polling is sufficient); rebind for loopback mode.
