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

**This breakdown cannot be reported for this campaign.** Root cause, found during analysis (not
before): `wire-and-score.sh` computes the full raw `score.sh` record (`contract_compliant`,
`compiled`, `post_fix_compiled`, `functional_pass`, `pre_count`, `post_count`, `residual`,
`clean`) per slot, but writes it only to a `/tmp/score-slot-NN-$$.json` scratch file that
`run-campaign.sh` deletes immediately after extracting the narrower `pass`/`mismatches`/
`infra_void` subset into the durable `journal/slot-NN.json` record. The raw fields were never
retained past each slot's own processing — this is a genuine gap in this cycle's harness design
(not inherited from `#91119`, which scored via a different, `pre_count`/`post_count`-free
mechanism entirely), discovered only after the campaign had already run to completion, when
retention would have needed to happen live, per-slot.

**Practical impact is bounded, not zero, but modest**: since every one of the 18 failures
failed at the `contract_compliant` gate (§ above) — before `score.sh`'s pipeline reaches the
`pre_count`/`post_count`/`residual` computation stage in any of them — the secondary breakdown's
informational content for THIS run would likely have been thin regardless (18 slots with the
same qualitative "never reached the transform stage" story; only the 2 passing slots would have
had a meaningful pre/post pair to report). This is offered as context for the gap's real cost,
not as a retroactive excuse for not having caught it before dispatch.

**Follow-up filed** to fix the retention gap for any future campaign reusing this harness (see
Deferred, below) — not re-run this campaign, since re-dispatching to recover secondary data would
violate the frozen no-repair/no-redispatch policy (§8 of the preregistration) and isn't necessary
to answer the primary causal question, which is now fully and validly answered above.

## UserSov §3c.5 — two-stage review

**Stage 1** (pre-dispatch, delta review across the 8 named surfaces): completed and published
(`tasks.jeeves` interaction events, this session, prior to slot 1 dispatch — retroactively
relative to slot 1 specifically, since a prior session's own harness-build work had already
dispatched it before this session's claim; disclosed honestly as a process gap in the prior
session, not hidden). Findings: property 9 (delegation integrity) — dojo's scoped mount +
`--hide` isolation verified directly against `dispatch.bash`'s actual code, disposition
`fixed-verified`. No other property yielded a live finding; all reasoned N/A or clear.

**Stage 2** (post-slot-1, before continuing to slots 2-20): reviewed slot 1's actual real record
against Stage 1's assumptions — held. No new sovereignty surface or violation surfaced by any
real record across the full 20-slot run. Clearance: proceed (published, campaign continued).

## Scope-fold: `#96719` hardening items

Neither of the two `#96719`-deferred hardening shapes (a genuinely ambiguous simultaneous
contract+build-failure output; a `transform-primary` panic/timeout/malformed-invocation) occurred
in this real 20-slot campaign — every failure was a clean, single-cause `contract_compliant`
miss with no infra ambiguity. Per `#96719`'s own stated preference, these items are carried
forward as a follow-up rather than authored as speculative fixtures now (nothing new to inform
them from this run).

## Deferred (tracked)

- **New follow-up (this cycle):** `wire-and-score.sh`/`run-campaign.sh`'s per-slot raw
  `score.sh` record (`contract_compliant`, `compiled`, `post_fix_compiled`, `functional_pass`,
  `pre_count`, `post_count`, `residual`) is discarded after each slot instead of being retained
  in the durable journal record — fix so any FUTURE campaign reusing this harness pattern can
  report the full secondary breakdown the preregistration's §6 calls for. Task to be filed at
  Completion Gate.
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
