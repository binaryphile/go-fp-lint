# #93569 dispatch harness

Executes the frozen randomized normal-brief-vs-technique-brief causal arm
preregistered at jeeves #91119
(`investigations/2026-08-01-model-routing-causal-arm-preregistration.md`).
The oracle (`../refcorrect.go`, `../refbroken.go`, `../score.go`,
`../testdata/`) is built and validated separately (#93569's earlier
integrity-gate step, commit `c9fc0bf`); this directory is steps 3–6:
dispatch, score, analyze, report — plus a two-stage UserSov review. Design
rationale, the R1–R4 adversarial grade trajectory (F→D→B→A), and the frozen
execution semantics are in the tandem plan
`~/.claude/plans/93569-idempotent-meandering-kitten.md`.

## Contents

| File | Role |
|---|---|
| `journal.bash` | Crash-safe, exactly-once per-slot state machine (`reserved→dispatched→captured→scored→recorded`). Immutable per-slot record files are the sole source of truth. |
| `scorer-control.bash` | Brackets each slot's scoring with a known-good reference; distinguishes "scorer/oracle broken" (infra-void, halt) from "delegate's own analyzer is wrong" (FAIL). |
| `dispatch.bash` | Fresh isolated pre-oracle clone per slot (`CloneAndIsolate`) + synchronous `claude -p` dispatch under `dojo` (`Run`). |
| `wire-and-score.sh` | Generates the ephemeral scoring module (`go.mod` replace-wiring against the PINNED oracle worktree), runs it, writes one atomic schema-validated result file. `go test`'s exit code is never trusted — only the result file is. |
| `parse_claude_result.py` | Raw-result adapter: extracts ONLY metric fields (`total_cost_usd`, `num_turns`, `usage.*`, `duration_ms`, `is_error`, `subtype`) from `claude -p --output-format json`'s object. Never touches free-text. |
| `stats.py` | Production exact statistics (Clopper–Pearson, two-sided Fisher, Newcombe hybrid-score) — pure Python (`math.comb`); no scipy/statsmodels (absent on this box). |
| `gen_stats_refs.py` → `stats_refs.json` | Independently-authored SECOND implementation (log-space Fisher, Newton-method CP) self-checked against externally-derivable closed-form anchors before emitting all 121 `(x_T,x_N)` reference pairs. `stats_test.py` validates production against this artifact — non-circular provenance. |
| `analysis-config.json` | Frozen analysis settings (alpha, CI method, Fisher sidedness, arm order, interpretation-rule text). |
| `FROZEN.json` | The freeze-before-dispatch manifest — every pin (briefs, model, CLI version, oracle commit+hash, sandbox config) recorded and committed BEFORE slot 1. |
| `briefs/arm-N.txt`, `arm-T.txt` | Frozen brief text, extracted byte-exact from the prereg's Appendix A/B (SHA-256 asserted at pre-flight). |
| `refpkgs/correct`, `refpkgs/broken` | The oracle's reference analyzers wrapped as the frozen delegate contract (package `fluentmap` @ `example.com/delegate/fluentmap`). Used for the integrity replay, scorer-control, and — `refpkgs/correct` doubling as the canned package — the mock-delegate dry-run. NOT delegate output. |
| `fixtures/claude-result-schema.json` | A real `claude -p --output-format json` result object, captured live from the exact pinned CLI+model, used to validate the adapter and as the mock-delegate's canned raw result. |
| `run-campaign.sh` | The driver: `runCampaign.preflight` (6 HALT-on-fail checks, zero vector consumption) then the 20-slot frozen App-D loop. |

## Running

Pre-flight only (safe; consumes no vector slots):

```bash
source run-campaign.sh
runCampaign.preflight ~/projects/go-fp-lint-oracle-c9fc0bf "$PWD/refpkgs/correct" "$PWD/refpkgs/broken"
```

Full campaign (real Haiku dispatch, real spend, one-shot):

```bash
bash run-campaign.sh journal results
```

`journal/` accumulates immutable per-slot records; a re-invocation after a
crash resumes at the first non-`recorded` slot (never re-dispatching a slot
past `reserved`) per the frozen recovery policy.

## Isolation notes

Each slot: `git clone --depth 1 --no-local --branch preoracle-base` (the
oracle-introducing commit's parent — verified absent), `git remote remove
origin` (kills the re-fetch path), dispatched under `dojo --hide
~/projects` (both the oracle worktree and the source repo become invisible)
with a fresh empty per-slot `GOMODCACHE` (network stays available — required
for the Anthropic API — so cross-slot module-cache contamination is
structurally impossible: each slot's cache is a brand-new directory,
discarded after the slot).

Scoring runs on the host, offline (`GOPROXY=off`), against a **pinned**
detached oracle worktree at `c9fc0bf` (never mutable `main`) whose clean
full-tree hash is verified before every scoring call.

## Khorikov posture (3c)

- `stats.py`/`gen_stats_refs.py` — **Calculation/Algorithm quadrant**: pure functions, no I/O, validated output-based via `stats_test.py` against an independently-provenanced reference table (121 pairs) plus externally-derivable closed-form anchors. Deterministic, parallelizable, safe to re-run.

- `journal.bash`, `scorer-control.bash`, `dispatch.bash`, `wire-and-score.sh`, `run-campaign.sh` — **Controller quadrant**: orchestrate subprocesses (git, dojo, claude, go test), mutate filesystem state (journal records, clones). Tested via **integration tests on the controller** (Khorikov §6275), not unit-mocked internals — `runCampaign.preflight`'s mock-delegate dry-run exercises the full discovery→scorer-control→score→classify→journal pipeline through the SAME code path a live slot uses, substituting only the dispatch step (a canned package + a captured real result fixture in place of a live `claude -p` call). This is the deliberate mock/live boundary (grade R2-9): everything downstream of "how the raw result JSON was produced" is shared code, exercised identically by both.

- `parse_claude_result.py` — thin adapter, validated directly against a live-captured fixture (`fixtures/claude-result-schema.json`).

No FAIL-stub-as-assertion patterns; `wire-and-score.sh`'s scorer test RECORDS pass/mismatches rather than asserting non-zero — a FAIL is a valid datum, not a test failure (prereg §8).

**Post-execution honesty (added after live runs):** the mock-delegate integration test did NOT catch two real defects that only manifested under live dispatch — (1) `dojo --project X` not changing the wrapped command's cwd (campaign-1, 6 slots, $1.73 wasted), and (2) `rm -rf` on a read-only Go module cache crashing the campaign under `set -e` (campaign-2, halted after slot 4). Both are genuine gaps in the Controller-layer test coverage: the mock path never exercises a real `dojo` invocation or a real populated `GOMODCACHE`, so it structurally could not have caught either. Both are now covered — (1) by the pre-flight mechanism check added at `run-campaign.sh` step 2/7 (zero-cost, no live `claude` call, verifies a file written by a dojo-wrapped command survives to the persisted dir); (2) by `chmod -R u+w` before every `rm -rf $slotGomodcache`. Recorded here rather than only in commit messages because it's a durable lesson about this Controller's test boundary, not a one-off fix.

## Campaign execution history

- **Campaign-1** (2026-08-02): voided after 6/20 slots — see `campaign-1-VOIDED/README.md`.
- **Campaign-2** (2026-08-02): completed all 20 slots but is operationally incomplete relative to the frozen n=10/arm (1 infra-void in arm N, orchestrator-caused mid-investigation kill, not a systemic recurrence). See `../RESULTS.md` for the full result and honesty framing.
- **Mandatory IMPL /grade** (2026-08-02, cross-vendor, Grade D → SEND BACK on first pass, findings absorbed): found six real post-execution defects the earlier review passes missed — the raw per-slot `claude` result files (containing the free-text `result` field, outside the S3 UserSov minimization allowlist) were never cleaned up, contradicting event #95018's `fixed-verified` disposition (fixed: `runCampaign.cleanupSlot` + a script-level EXIT trap); the crash-recovery path (`journal.KillAndProve`) searched the wrong clone path and could have falsely proven a live process dead (fixed: use the real `/tmp/slot-NN-clone` path); the same recovery branch recorded the terminal `infra-void` state BEFORE confirming proof, so a failed proof still left the slot permanently closed despite the intended halt (fixed: reordered); the exclusive campaign lock (`journal.Lock`) was defined but never called from `main()` (fixed: wired in); `journal.Record` accepted a second call and silently overwrote a terminal record (fixed: refuses once `recorded`); and the result-file consumer trusted `.pass`/`.mismatches` without checking they were actually present with the right types (fixed: `jq -e` shape validation before trusting the file). All fixes verified via the pre-flight suite; all already-leaked raw-result files removed from `/tmp`. RESULTS.md also had unfilled template placeholders in its headline claim and an overclaimed causal attribution for the `no_matching_module` failures — both corrected.
