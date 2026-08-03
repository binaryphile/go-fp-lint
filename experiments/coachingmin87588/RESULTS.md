# Coaching-Minimum Causal Arm — Results

**Preregistration:** `jeeves/investigations/2026-08-02-transform-primary-coaching-minimum-preregistration.md`

**Task:** jeeves#96780 (harness fork + real dispatch, forked from `#91119`/`#93569`'s pattern)

**Frozen protocol:** oracle pinned at `go-fp-lint@16991c9`, vehicle `SummarizeActiveUsers`,
model `claude-haiku-4-5-20251001`, seed 87588, randomization vector `CCCUCCUCUCUUUUUCUCUC`
(slot 1→C, 2→C, 3→C, 4→U, 5→C, 6→C, 7→U, 8→C, 9→U, 10→C, 11→U, 12→U, 13→U, 14→U, 15→U, 16→C,
17→U, 18→C, 19→U, 20→C).

## Campaign execution

- **Pilot-before-dispatch check (§7): PASSED both required criteria** before any real dispatch —
  pilot-clean (pre_count=1, post_count=0, residual=false) and pilot-residual (pre_count=1,
  post_count=1, residual=true), the exact expected shapes.
- **20/20 slots dispatched and recorded.** No campaign halt beyond the mandatory, planned
  Stage-2 UserSov pause after slot 1 (cleared after review — see §3c.5 below).
- **infra-void count: 0.** No `post_fix_compiled:false` occurrences in either arm — the
  halt-on-pattern trigger (2+ occurrences) never fired, and the single-occurrence
  reduced-denominator caveat does not apply. Full n=10/arm reached cleanly.
- **Real spend: $4.997646 total** across 20 real delegate dispatches (per-slot costs
  $0.13–$0.64, `claude-haiku-4-5-20251001`).

## Primary endpoint (frozen, binary ITT, n=10/arm reached)

| Arm | Pass (clean) | Fail (not-clean) | Clopper-Pearson 95% CI |
|---|---|---|---|
| C (coached) | 1/10 | 9/10 | (0.0025, 0.4450) |
| U (uncoached) | 1/10 | 9/10 | (0.0025, 0.4450) |

**Fisher's exact (two-sided): p = 1.0000.**

**Difference (C − U) 95% CI (Newcombe hybrid-score): (−0.3150, 0.3150).**

**Fixed interpretation (per the preregistration's own frozen rule, applied mechanically —
not post-hoc-narrated in either direction):** not significant → **underpowered / no detectable
effect at n=10 (NOT evidence of no effect).** The pass rate is identical between arms (1/10
each); the coaching block produced no directionally distinguishable effect on the ITT-clean
endpoint at this sample size.

**Explicit CI-width caveat (IMPL-grade finding):** the difference CI half-width (~0.315) is
wide enough to be compatible with a practically meaningful true effect in either direction —
"underpowered" is not a claim that no such effect exists, only that n=10/arm cannot distinguish
it from zero here. A true difference of, say, 20-30 percentage points between arms would still
be statistically indistinguishable from the observed identical 1/10 result at this sample size.

**Failure mode, both arms:** all 18 failing slots failed at the SAME gate —
`mismatches: ["contract_compliant"]` — i.e., the delegate's code never satisfied the
frozen structural contract (using `fluentfp`'s `KeepIf`/`Map` chain methods rather than a
hand-written loop), regardless of arm. Zero slots in either arm failed later in the pipeline
(compile, post-fix-compile, functional, or residual) — every failure was a contract-shape
miss, not a functional or transform-behavior miss.

## Secondary breakdown — GAP, honestly disclosed (not fabricated, not silently omitted)

The preregistration's §6 calls for a secondary, explicitly non-causal descriptive breakdown
(functional-pass rate by arm; raw `pre_count`/`post_count` by arm; residual-rate conditional on
functional pass) — informative context, never substituted for the primary endpoint above.

**This breakdown cannot be reported for this campaign — for two DIFFERENT reasons depending on
outcome, not one uniform cause (IMPL-grade R2 correction of an internally inconsistent earlier
draft):**

1. **For the 18 `contract_compliant:false` slots, the data was never GENERATED at all**, not
   discarded after computation. `score.sh` (lines 63-66) returns early the moment
   `contract_compliant != true`, emitting literal `null` for `pre_count`/`post_count`/`residual`
   and never calling `transform-primary --only nestedcall` (line 69) in the first place. The
   pipeline's own short-circuit design means these 18 measurements simply do not exist to be
   retained or lost.
2. **For the 2 passing slots, the data WAS genuinely computed** (score.sh's full pipeline ran to
   completion) **but then discarded by the harness**: `wire-and-score.sh` writes the complete raw
   record to a `/tmp/score-slot-NN-$$.json` scratch file that `run-campaign.sh` deletes
   immediately after extracting only the narrower `pass`/`mismatches`/`infra_void` subset into the
   durable `journal/slot-NN.json` record. This part IS a retention gap in this cycle's harness
   design (not inherited from `#91119`, which scored via a different, `pre_count`/`post_count`-free
   mechanism entirely) — discovered only after the campaign had already run to completion, when
   retention would have needed to happen live, per-slot.

**Net effect**: even a full retention fix (the follow-up below) would only recover secondary data
for the 2 passing slots per arm-equivalent, not the 18 failing ones — those would need `score.sh`
itself changed to compute `pre_count`/`post_count` independently of `contract_compliant`'s
short-circuit, a separate and larger change than a retention fix alone, not undertaken here
(would itself require its own plan/review, since it changes the frozen scoring contract).

**This is a protocol deviation, not a minor omission (IMPL-grade correction).** The
preregistration's §6 secondary breakdown is REQUIRED reporting, not optional context — its
unavailability is recorded here as **UNAVAILABLE / lost**, full stop. An earlier draft of this
section speculated that the missing data's informational value "would likely have been thin"
because all 18 failures were `contract_compliant` misses. That speculation is withdrawn on
review: binary contract-pass/fail does not establish what the PRE/POST diagnostic counts would
have shown about severity or mechanism even among the failing slots (the contract check and the
`transform-primary` diagnostic counts are independent measurements on the same candidate code —
a contract-noncompliant candidate can still be run through `--only nestedcall` and produce a
real pre/post count; `score.sh`'s own short-circuit design chose not to compute those fields once
`contract_compliant` was false, which is a pipeline design decision, not evidence the numbers
would have been uninformative). The honest position is: the secondary result is unavailable, its
value is unknown, and no impact assessment is offered in its place.

**Root-cause classification**: this is also a preflight-adequacy defect, not merely a
retention-format oversight. The pre-flight checks (PRE-FLIGHT 1/7-7/7, all passed before real
dispatch) verified the scorer's raw OUTPUT correctness but never verified that the REQUIRED
analysis fields survive the full dispatch → journal → analysis persistence path end-to-end
before real money was spent. That path-completeness check should have been part of pre-flight
and was not — recorded here as the clearest implementation defect in this cycle, not softened.

**Follow-up filed** (`go-fp-lint#98182`) to fix the retention gap AND add a pre-flight check that
verifies the full persistence path (not just scorer-output correctness) before any future
campaign's real dispatch. Not re-running this campaign to recover secondary data, since
re-dispatching would violate the frozen no-repair/no-redispatch policy (§8 of the
preregistration) and the primary causal question is already fully and validly answered above —
but the secondary result itself stays recorded as lost, not estimated.

## UserSov §3c.5 — two-stage review

**Protocol deviation, recorded explicitly (IMPL-grade finding, not self-cleared).** The plan
requires the Stage-1 11-property record to exist BEFORE slot 1 dispatches. It did not: a prior
session's harness-build work dispatched slot 1 (real $0.14 spent) before any Stage-1 record was
ever published. This session discovered the gap on pickup, after slot 1 had already run, and
authored Stage 1 retroactively as the best available mitigation — but authoring a "prior"
baseline after the fact does not recreate a genuine independent-prior review, and the campaign
is **not fully process-conformant** on this point. This is disclosed as a real deviation, not
papered over as equivalent to a compliant run. `evtctl interaction "/variance"` filed on
`tasks.jeeves` recording the deviation explicitly (see completion-gate evidence).

**Stage 1** (delta review across the 8 named surfaces, authored retroactively per above):
property 9 (delegation integrity) — dojo's scoped mount + `--hide` isolation verified directly
against `dispatch.bash`'s actual code, disposition `fixed-verified`. No other property yielded a
live finding; all reasoned N/A or clear.

**Stage 2** (post-slot-1, before continuing to slots 2-20 — this part of the design WAS followed
correctly, since slot 1 genuinely paused for review before slot 2 ran): reviewed slot 1's actual
real record against the (retroactive) Stage-1 baseline — held. No new sovereignty surface or
violation surfaced by any real record across the full 20-slot run. Clearance: proceed (published,
campaign continued).

**Durable evidence**: the full per-slot journal (`journal/slot-01.json` through `slot-20.json`,
`journal/campaign.json`) is committed alongside this file — not left as local-only, untracked
state — so slot outcomes, real costs, and the absence of any halt-condition trigger beyond the
planned Stage-2 pause are independently auditable from the repository itself, not only from this
narrative (IMPL-grade finding: a real-spend, high-risk campaign's evidence should not rely solely
on prose recounting).

## Phase 3c Khorikov posture — bash conformance note

The one genuinely new file (`wire-and-score.sh`) is zero-SC9xxx-clean. The three forked files
(`dispatch.bash`, `journal.bash`, `scorer-control.bash`) carry pre-existing SC9xxx findings
inherited unchanged from `#91119`'s already-shipped source (verified by direct diff:
`dispatch.bash` differs by one hardcoded path string, `journal.bash` by a doc-comment only,
`scorer-control.bash` is byte-identical). **IMPL-grade caveat, recorded rather than
self-adjudicated**: no explicit, recorded estate-wide policy currently establishes that a forked
file's pre-existing debt is exempt from this cycle's own conformance responsibility merely
because the diff is small — this cycle's judgment that "unchanged inherited code is out of
scope" is a reasonable per-cycle call, not a citation of an established convention. Recorded
here as an open question for tandem-protocol's own bash-conformance discipline to resolve
generally, rather than asserted as settled by this cycle alone.

## Scope-fold: `#96719` hardening items

Neither of the two `#96719`-deferred hardening shapes (a genuinely ambiguous simultaneous
contract+build-failure output; a `transform-primary` panic/timeout/malformed-invocation) occurred
in this real 20-slot campaign — every failure was a clean, single-cause `contract_compliant`
miss with no infra ambiguity. Per `#96719`'s own stated preference, these items are carried
forward as a follow-up rather than authored as speculative fixtures now (nothing new to inform
them from this run).

## Deferred (tracked)

- **Filed this cycle: `go-fp-lint#98182`.** For the 2 passing-slot case, `wire-and-score.sh`/
  `run-campaign.sh`'s per-slot raw `score.sh` record is discarded after each slot instead of
  being retained in the durable journal record — fix so any FUTURE campaign reusing this harness
  pattern can report the full secondary breakdown for slots that reach the transform stage. Note:
  this fix alone does NOT recover data for `contract_compliant:false` slots (see Secondary
  breakdown § above) — that would require changing `score.sh`'s own short-circuit design, a
  separate, larger, frozen-contract change not undertaken by this follow-up.
- `#96719`'s two hardening items — still carried forward, uninformed by this run (see above).

## Honest summary

The frozen causal question — does an upfront coaching block on `fluentfp`'s chain-method
contract change a delegate's post-transform residual-violation rate on this vehicle — is
**answered inconclusively at n=10/arm**: identical 1/10 pass rate in both arms, Fisher p=1.0,
zero detectable effect. This is a predeclared, honest, likely-modal outcome (per the
preregistration's own §8 framing), not a failure of the experiment design. The dominant real
finding is that BOTH arms overwhelmingly failed on contract-shape compliance (18/20 slots) — a
`SummarizeActiveUsers`-vehicle-specific difficulty for this model that swamped whatever
coaching-related signal might exist at the transform-behavior layer, and that a coaching block
about the contract's PROSE requirement did not measurably move.
