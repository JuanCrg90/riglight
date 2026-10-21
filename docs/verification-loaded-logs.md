# Follow-loaded-model log verification

## Scope

Owner requested automatic following of the single currently loaded model rather
than explicit model mappings. Implementation follows
[the extension contract](spec-loaded-logs.md). Logs remain disabled by default;
no production log content, real inventory probes, model actions, installed service
configuration, permissions, listener policy or deployed binaries changed.

The new flag requires --logs and --llama-swap and excludes --log-model. The
stable LOADED_MODEL source uses the existing inventory hub, not additional
upstream discovery or a second browser metric stream. A fresh, complete, unique
reported LOADED target is selected with a monotonic six-second expiry. Stale,
missing, ambiguous, truncated and unsupported targets fail closed. Loaded state
and output do not establish readiness.

## Independent review and fixes

The primary reviewed the architecture and source; an independent GPT contract
review approved it before implementation. Separate GPT Go/security and frontend
reviews were performed, with fixes owned by the original exclusive writer.

- Initial Go review and primary inspection identified EOF-tail loss, expiry
  racing a fresh same-target sample, stale lines selected after a blocking wait,
  cancellation priority and target-envelope enforcement gaps. Tests and fixes
  drain valid EOF tails, reselect immutable latest inventory, recheck freshness
  after line receipt, enforce first revision/readiness/FOLLOWING server-side, and
  stop before opening a stale replacement reader.
- Shared raw input budgets are not renewed by model changes, including a raw-cap
  race with the next inventory reporting no loaded model. No budget lock is read
  by the controlling loop while the child is blocked in body Read; cancellation
  and join precede the check.
- Frontend review found reserved model references accepted by the target parser;
  proxy/upstream/http are now rejected like the backend, with parser and UI
  fail-closed tests. Cross-stack review additionally identified overly narrow
  frontend matching of dotted/leading-punctuation safe identifiers; parser and UI
  tests align it exactly with the backend, excluding exact dot segments rather
  than internal dots. Source kind determines behavior, not an ID match.
- Primary additionally checked recovery if a target announcement blocks while
  inventory changes, so an announced but not yet attached target cannot stall
  permanently on a subsequent same-target refresh.

## Executed synthetic gates

Pinned Go 1.27.3, Node 24.22.0 and pnpm 12.14.0; no added external dependency,
license family or module/lockfile change. Existing upstream native NVML deprecated
API compiler warnings were not suppressed and do not indicate a failed check.

Commands in the final pipeline:

```sh
mise exec -- pnpm --dir web install --offline --frozen-lockfile --ignore-scripts
mise exec -- pnpm --dir web test
mise exec -- pnpm --dir web lint
mise exec -- pnpm --dir web typecheck
mise exec -- pnpm --dir web build
mise exec -- go mod verify
test -z "$(mise exec -- gofmt -l cmd internal)"
mise exec -- go test -race ./...
mise exec -- go vet ./...
mise exec -- go build -trimpath -o bin/riglight ./cmd/riglight
mise exec -- go build -trimpath -o bin/riglight-docker-proxy ./cmd/riglight-docker-proxy
CGO_ENABLED=0 mise exec -- go test ./...
CGO_ENABLED=0 mise exec -- go build -trimpath -o bin/riglight-nocgo ./cmd/riglight
CGO_ENABLED=0 mise exec -- go build -trimpath -o bin/riglight-docker-proxy-nocgo ./cmd/riglight-docker-proxy
python3 -B -m unittest discover -s deploy/systemd -p 'test_*.py' -v
mise exec -- pnpm --dir web exec playwright test --project=chromium --project=chromium-gpu --project=chromium-models --project=chromium-containers --project=chromium-system --project=chromium-logs --project=chromium-loaded-logs --workers=2
mise exec -- pnpm --dir web audit
git diff --check
```

The complete final integrated run after all Go and frontend fixes passed 197
frontend tests, six public systemd example tests and 50 Chromium checks. A built-helper fake Unix-daemon
exercise passed selected/framed/TTY/UTF-8/inspect-sentinel/guard/shutdown checks
without real Docker access. govulncheck and pnpm audit reported no known
vulnerabilities. The formatted-source check returned no files; race, vet, native/
no-CGO tests and all four builds passed. The dotted-identifier browser regression
was first demonstrated failing against the old parser (empty log output for a
valid target), then passed in the full final run.

Six new Chromium cases exercise actual synthetic automatic-demo output, initial
source preference and explicit Start, A->waiting->B transitions on one log/metric
connection, ambiguity/unavailability/unsupported states, line-before-target and
cross-target line-sequence rejection, paused/resumed model labels and old callback
isolation, plus malformed/foreign/replayed/reserved target envelopes. Existing
44 checks still pass. No Safari/WebKit result is required or claimed.

Go tests use injectable synthetic clocks, inventory hubs, reader factories and
barriers, covering fresh/no-target/ambiguous/stale/unsupported selection, expiry
without publication, no same-target reopen, ordered cancellation/reader joining,
aggregate raw budgets, finite EOF tails, continuous-update cancellation and
post-receive freshness. Main server tests assert exact target envelope mode/
source/revision/null/ready/FOLLOWING validation and cumulative session limits.

## Handoff gates and limitations

Final documentation/link/fence/shell, dependency/linked-graph and privacy scans
passed, including full Git history/public export/frontend Gitleaks and complete
source/export/frontend/four-binary operator-identifier/path/source-map checks.
Manifests, lockfiles, module files and notices remained byte-identical to the
increment baseline; the dashboard's linked graph still excludes the privileged
Docker helper package. Independent Go, frontend and final combined review
findings were fixed and the code approved before handoff.
The previously reviewed exact-fingerprint historical prose exception remains
unchanged; no new scanner rule exclusion is introduced.

No live log endpoint was read. Installed llama-swap version/model-reference
compatibility is not claimed. Slash/reserved/unsupported model IDs stop following,
not route to a broader channel. Initial upstream bounded history may repeat on
Resume or a later reattachment; no model-process incarnation/catch-up guarantee.
EOF, errors and five-minute/2-MiB/4096-line session limits remain terminal; model
changes do not reset them. Raw logs are not guaranteed redacted and retain the
existing admitted dashboard audience. Activation needs a separately approved
binary/configuration rollout, now without manual per-model mappings. Existing
persistent services must not be restarted as a coding/verification side effect.
No reboot or publication was performed.
