#!/usr/bin/env python3
"""analyze.py — the frozen §8 analysis over a completed campaign's journal
(prereg §8; plan "Infra-void terminal semantics", grade R3-1).

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

    n_by_arm = {"T": 0, "N": 0}
    pass_by_arm = {"T": 0, "N": 0}
    infra_void = []
    fails_by_arm = {"T": [], "N": []}

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

    xT, nT = pass_by_arm["T"], n_by_arm["T"]
    xN, nN = pass_by_arm["N"], n_by_arm["N"]

    frozen_n = config["n_per_arm_frozen"]
    complete = (len(infra_void) == 0) and (nT == frozen_n) and (nN == frozen_n)

    result = {
        "complete": complete,
        "infra_void_count": len(infra_void),
        "infra_void_slots": infra_void,
        "n_T": nT, "n_N": nN,
        "pass_T": xT, "pass_N": xN,
        "fails_T": fails_by_arm["T"], "fails_N": fails_by_arm["N"],
    }

    if not complete:
        result["framing"] = (
            "OPERATIONALLY INCOMPLETE / ABORTED PILOT — this campaign did NOT reach the "
            f"preregistered n={frozen_n}/arm (T: {nT}/{frozen_n}, N: {nN}/{frozen_n}, "
            f"{len(infra_void)} infra-void). Per the frozen no-repair/no-backfill policy, this "
            "result is reported on the ACTUAL completed n, never as the preregistered full-n "
            "causal result."
        )
        # Still compute descriptive stats on the actual completed n, honestly labeled.
        if nT > 0 and nN > 0:
            result["descriptive_fisher_p"] = stats.fisher_exact_two_sided(xT, xN, n_per_arm=max(nT, nN)) \
                if nT == nN else None
        with open(os.path.join(results_dir, "summary.json"), "w") as f:
            json.dump(result, f, indent=2)
        print(result["framing"])
        print(json.dumps(result, indent=2))
        return

    # Complete: n_T == n_N == frozen_n == 10. Full frozen §8 analysis.
    fisher_p = stats.fisher_exact_two_sided(xT, xN)
    cp_T = stats.clopper_pearson(xT, nT)
    cp_N = stats.clopper_pearson(xN, nN)
    diff_ci = stats.newcombe_diff_ci(xT, xN, nT, nN)

    alpha = config["alpha"]
    significant = fisher_p < alpha
    if significant and xT > xN:
        interpretation = config["interpretation"]["significant_T_gt_N"]
    elif significant and xN > xT:
        interpretation = config["interpretation"]["significant_N_gt_T"]
    else:
        interpretation = config["interpretation"]["not_significant"]

    result.update({
        "framing": "PREREGISTERED RESULT (n=10/arm reached, 0 infra-void)",
        "fisher_p_two_sided": fisher_p,
        "cp_T_95ci": list(cp_T),
        "cp_N_95ci": list(cp_N),
        "diff_T_minus_N_95ci": list(diff_ci),
        "significant_at_alpha": significant,
        "alpha": alpha,
        "interpretation": interpretation,
    })

    with open(os.path.join(results_dir, "summary.json"), "w") as f:
        json.dump(result, f, indent=2)

    print(f"T: {xT}/{nT} pass  CP95%={cp_T}")
    print(f"N: {xN}/{nN} pass  CP95%={cp_N}")
    print(f"Fisher two-sided p={fisher_p:.4f}  diff(T-N) 95% CI={diff_ci}")
    print(f"Interpretation: {interpretation}")


if __name__ == "__main__":
    main()
