# #93569 causal-arm campaign — results

Executes the frozen randomized normal-brief-vs-technique-brief causal arm
preregistered at jeeves #91119
(`investigations/2026-08-01-model-routing-causal-arm-preregistration.md`).
Full harness design, the R1–R4 adversarial grade trajectory (F→D→B→A), and
the frozen execution semantics: `harness/README.md` and the tandem plan
`~/.claude/plans/93569-idempotent-meandering-kitten.md`.

## Strongest defensible claim (prereg §1, restated)

> *Given the frozen briefs (App A/B) and the frozen hidden oracle (App C), on
> the single go-fp-lint type-aware vehicle of §3, at the ACTUAL completed n
> (T: [x/10], N: [y/9] — see "Campaign status" below, this run did NOT reach
> the preregistered n=10/arm): the technique-explicit brief variant's pass
> rate on THIS vehicle was [x] vs the normal brief variant's [y].*

## What is forbidden in this report (prereg §1, unchanged)

- "Technique briefing makes cheap-tier execution reliable" — not licensed.
- Attributing any effect to "technique content" specifically — the treatment
  is the whole brief **bundle** (technique content + actionable disclosure +
  test-authoring hints), never isolated technique content.
- Any generality claim about Haiku, about briefing across task families, or
  about "routing."
- Any savings/economics claim (frozen at #91148, never inferred here).
- Post-hoc re-narration of an inconclusive/floor result as support in either
  direction.
- Percentages at this n (report counts).

## Campaign status: OPERATIONALLY INCOMPLETE (not the preregistered full-n result)

Two live campaigns ran. **Campaign-1** (2026-08-02) halted after 6/20 slots:
a harness defect (`dojo --project X` does not change the wrapped command's
cwd) caused 5 real, substantial delegate deliveries to land outside the
persisted/discoverable directory, misclassified as delivery FAIL. Voided
entirely — not used as data (archived at `harness/campaign-1-VOIDED/` for
root-cause transparency only). Cost: $1.73.

**Campaign-2** (2026-08-02), under the fixed and re-verified harness (a new
zero-cost pre-flight mechanism check now guards against recurrence), ran to
completion — all 20 slots reached a terminal journal state. **But it is
still not the frozen n=10/arm result**: slot 3 (arm N) went `infra-void`
when the orchestrator killed the background run mid-dispatch to investigate
an unrelated discovery-rate question (verified no process survived; per the
frozen no-repair/no-backfill policy this is excluded and counted, never
backfilled — see journal `slot-03.json`, reason
`driver_killed_mid_dispatch_for_investigation`).

Per the frozen infra-void terminal-semantics rule (tandem plan, absorbed
from grade R3-1): **any nonzero infra-void count means this campaign is
reported on the ACTUAL completed n, never as the preregistered full-n causal
result.**

| Arm | Frozen n | Actual completed n | Pass |
|---|---|---|---|
| T (technique) | 10 | **10** | **0** |
| N (normal) | 10 | **9** (1 infra-void) | **0** |

**Zero passes in either arm.** No Fisher/CI computation is reported (the
frozen §8 machinery requires n=10/arm to be meaningful under the prereg's own
predeclared analysis plan; at 0/10 vs 0/9 it would show no difference by
construction, which duplicates the count table without adding information).

## Why zero passes — this is real, mechanically-explicable data, not a harness artifact

Every scoreable slot's failure was individually inspected (not just counted)
before this report was written, specifically to rule out a residual harness
bug given campaign-1's history:

- **10/19 valid slots ({1,2,7,9,10,11,12,15,17,18} — 5 T, 5 N)**: `delegate
  module not discoverable` (`no_matching_module`). Verified via an out-of-band
  diagnostic dispatch (real frozen brief text, not scored against the vector)
  that the fixed harness DOES correctly discover a delegate's module when one
  is placed in the expected location — this is genuine delegate placement
  variance across a 15–88 turn autonomous coding session, not a recurrence of
  campaign-1's bug.
- **3/19 valid slots ({8,19,20} — 2 T, 1 N)**: delegate module found, but
  scoring produced no result file after a healthy scorer-control bracket
  (`no_result_file_scorer_healthy`) — the delegate's own analyzer failed to
  compile or panicked (correctly classified FAIL, not infra-void, per the
  frozen classifier).
- **6/19 valid slots ({4,5,6,13,14,16} — 2 T, 4 N)**: delegate module found
  AND scored, but failed the oracle's **exact-message** check
  (`scored_not_pass`). Inspecting the mismatches directly: delegates
  correctly identified the right call sites (including hard cases — type
  aliases, embedded/promoted methods) but consistently printed the receiver
  type using Go's default `String()` formatting, which includes generic
  instantiation (`Mapper[int]`) or a bare short name (`Mapper`), rather than
  the App-C spec's exact `github.com/binaryphile/fluentfp/slice.Mapper` (no
  brackets, full package path). This pattern recurs independently across
  slots in BOTH arms — a genuine, oracle-enforced strictness trap (handling
  Go's generic type-string formatting correctly), not a brief-dependent
  effect.

None of the three failure classes shows an arm-dependent pattern under visual
inspection; both arms hit all three classes at comparable rates. This
vehicle, at this oracle's strictness, appears to sit at a floor for Haiku 4.5
regardless of brief variant — consistent with the prereg's own predeclared
framing that an honest, informative null/floor result is a legitimate,
likely-modal outcome at n=10/arm (§7), not a failed experiment.

## Threats to validity (prereg §9, unchanged) plus one added by this run

All six threats named in the prereg (operator-authoring bias, single vehicle,
single model, technique-vs-actionable-disclosure co-variation, brief-pair
asymmetry, latent oracle blind spot) apply unchanged. Added by this
execution: **the n-completeness threat** — this run's own infra-void
(orchestrator-caused, not a systemic recurrence) means even the descriptive
counts above are one slot short of the frozen design in arm N, further
limiting what can be concluded beyond "no pass observed in either arm at
this depth."

## Cost

Campaign-1 (voided): $1.73. Campaign-2 (this result): $5.52 (sum of
`total_cost_usd` across the 9 slots where discovery succeeded; the 10
`no_matching_module` slots have no captured cost — the S3 minimization
design extracts metrics only AFTER successful discovery, so those slots'
dispatch cost is un-metered in this report, a real reporting gap for any
future rerun of this harness, not a UserSov concern — no data was
over-captured, only under-captured relative to the full §10 metric vector).
Total this cycle: $7.25, plus prior diagnostic dispatches (~$1.35) = ~$8.6
all-in.

## UserSov disposition

Two-stage review passed (tasks.jeeves events #95016–95018 Stage 1,
#95355 Stage 2). Result-stream expungability (property 8) disposition:
**stream + scheduled expiry** — `experiment.causalarm91119` (20 `slot-result`
events, ids in the 95356–95375 range) is scheduled for `era trim` 90 days
from this campaign's completion (2026-08-02), i.e. on or after **2026-10-31**:

```bash
era trim --before "2026-10-31" -s experiment.causalarm91119 --confirm
```

The local `results/slots.jsonl` and `harness/journal/` are `rm`-deletable at
any time. Campaign-1's voided journal (`harness/campaign-1-VOIDED/`) is
retained indefinitely as root-cause-audit documentation, not experimental
data, and carries no expiry commitment.

## Bottom line

**This arm did not produce a usable pass-rate comparison.** At the actual
completed n (T:0/10, N:0/9), neither brief variant enabled Haiku 4.5 to fully
satisfy this oracle's strict exact-message contract on this vehicle. The
observed failure modes (module discoverability variance, compile/panic
failures, and — most informatively — a consistent generic-type-formatting
miss on the exact-message check) are real, individually-verified, and
arm-independent. Whether a brief variant that explicitly warns about Go's
generic type-string formatting would clear this floor is an open question
this run cannot answer and this report does not speculate on.
