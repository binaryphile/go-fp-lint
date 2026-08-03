#!/usr/bin/env python3
"""analyze.py — the frozen §8 analysis over a completed campaign's journal
(jeeves #96780, forked from #93569's causalarm91119/harness/analyze.py with
only the T/N -> C/U arm-letter rename; no other logic changes).

Reads journal/slot-NN.json (the sole source of truth), applies the frozen
analysis-config.json, and emits results/summary.json + a human-readable text
summary. Derives slots.jsonl from the journal as a side effect (never an
independent write — grade R2-11's "no dual-write race").

CRITICAL rule (R3-1): n is frozen at 10/arm with no re-dispatch. If ANY slot
is infra-void, the campaign is operationally INCOMPLETE — this script computes
on the ACTUAL completed n per arm and labels the result incomplete/aborted; it
NEVER presents a reduced-n run as the preregistered full-n result. Only
infra_void_count == 0 licenses the "preregistered result" framing.

Usage: analyze.py JOURNAL_DIR RESULTS_DIR
"""

import glob
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import stats


def load_slots(journal_dir):
    slots = []
    for f in sorted(glob.glob(os.path.join(journal_dir, "slot-*.json"))):
        with open(f) as fh:
            slots.append(json.load(fh))
    return slots


def main():
    if len(sys.argv) != 3:
        print("usage: analyze.py JOURNAL_DIR RESULTS_DIR", file=sys.stderr)
        sys.exit(1)
    journal_dir, results_dir = sys.argv[1], sys.argv[2]
    os.makedirs(results_dir, exist_ok=True)

    with open(os.path.join(HERE, "analysis-config.json")) as f:
        config = json.load(f)

    slots = load_slots(journal_dir)
    unrecorded = [s for s in slots if s.get("state") != "recorded"]
    if unrecorded:
        print(f"FATAL: {len(unrecorded)} slot(s) not in 'recorded' state — campaign incomplete, "
              f"cannot analyze: {[s.get('slot') for s in unrecorded]}", file=sys.stderr)
        sys.exit(1)

    # Derive slots.jsonl from the journal (the journal is the sole truth).
    slots_jsonl_path = os.path.join(results_dir, "slots.jsonl")
    with open(slots_jsonl_path, "w") as f:
        for s in slots:
            f.write(json.dumps(s) + "\n")

    n_by_arm = {"C": 0, "U": 0}
    pass_by_arm = {"C": 0, "U": 0}
    infra_void = []
    fails_by_arm = {"C": [], "U": []}

    for s in slots:
        arm = s["arm"]
        status = s["status"]
        if status == "infra-void":
            infra_void.append({"slot": s["slot"], "arm": arm, "reason": s.get("reason")})
            continue
        n_by_arm[arm] += 1
        if status == "pass":
            pass_by_arm[arm] += 1
        elif status == "fail":
            fails_by_arm[arm].append({"slot": s["slot"], "reason": s.get("reason")})

    xC, nC = pass_by_arm["C"], n_by_arm["C"]
    xU, nU = pass_by_arm["U"], n_by_arm["U"]

    frozen_n = config["n_per_arm_frozen"]
    complete = (len(infra_void) == 0) and (nC == frozen_n) and (nU == frozen_n)

    result = {
        "complete": complete,
        "infra_void_count": len(infra_void),
        "infra_void_slots": infra_void,
        "n_C": nC, "n_U": nU,
        "pass_C": xC, "pass_U": xU,
        "fails_C": fails_by_arm["C"], "fails_U": fails_by_arm["U"],
    }

    if not complete:
        result["framing"] = (
            "OPERATIONALLY INCOMPLETE / ABORTED PILOT — this campaign did NOT reach the "
            f"preregistered n={frozen_n}/arm (C: {nC}/{frozen_n}, U: {nU}/{frozen_n}, "
            f"{len(infra_void)} infra-void). Per the frozen no-repair/no-backfill policy, this "
            "result is reported on the ACTUAL completed n, never as the preregistered full-n "
            "causal result."
        )
        # Still compute descriptive stats on the actual completed n, honestly labeled.
        if nC > 0 and nU > 0:
            result["descriptive_fisher_p"] = stats.fisher_exact_two_sided(xC, xU, n_per_arm=max(nC, nU)) \
                if nC == nU else None
        with open(os.path.join(results_dir, "summary.json"), "w") as f:
            json.dump(result, f, indent=2)
        print(result["framing"])
        print(json.dumps(result, indent=2))
        return

    # Complete: n_C == n_U == frozen_n == 10. Full frozen §8 analysis.
    fisher_p = stats.fisher_exact_two_sided(xC, xU)
    cp_C = stats.clopper_pearson(xC, nC)
    cp_U = stats.clopper_pearson(xU, nU)
    diff_ci = stats.newcombe_diff_ci(xC, xU, nC, nU)

    alpha = config["alpha"]
    significant = fisher_p < alpha
    if significant and xC > xU:
        interpretation = config["interpretation"]["significant_C_gt_U"]
    elif significant and xU > xC:
        interpretation = config["interpretation"]["significant_U_gt_C"]
    else:
        interpretation = config["interpretation"]["not_significant"]

    result.update({
        "framing": "PREREGISTERED RESULT (n=10/arm reached, 0 infra-void)",
        "fisher_p_two_sided": fisher_p,
        "cp_C_95ci": list(cp_C),
        "cp_U_95ci": list(cp_U),
        "diff_C_minus_U_95ci": list(diff_ci),
        "significant_at_alpha": significant,
        "alpha": alpha,
        "interpretation": interpretation,
    })

    with open(os.path.join(results_dir, "summary.json"), "w") as f:
        json.dump(result, f, indent=2)

    print(f"C: {xC}/{nC} pass  CP95%={cp_C}")
    print(f"U: {xU}/{nU} pass  CP95%={cp_U}")
    print(f"Fisher two-sided p={fisher_p:.4f}  diff(C-U) 95% CI={diff_ci}")
    print(f"Interpretation: {interpretation}")


if __name__ == "__main__":
    main()
