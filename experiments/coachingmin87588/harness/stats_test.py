#!/usr/bin/env python3
"""Validate production stats.py against the independently-generated frozen
reference table (stats_refs.json) across ALL 121 (x_T, x_N) pairs, plus
externally-derivable anchors and structural properties (prereg §8; grade
R2-7/R2-11).

Exit 0 = all pass; nonzero = failure (blocks the campaign pre-flight).
"""

import json
import os
import sys
from math import comb

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import stats  # production

TOL = 1e-6
HERE = os.path.dirname(os.path.abspath(__file__))


def approx(a, b, tol=TOL):
    return abs(a - b) <= tol


def main():
    with open(os.path.join(HERE, "stats_refs.json")) as f:
        refs = json.load(f)

    fails = []

    # 1) production matches the independent 121-pair table
    for row in refs["table"]:
        xT, xN = row["xT"], row["xN"]
        fp = stats.fisher_exact_two_sided(xT, xN)
        if not approx(fp, row["fisher_p"]):
            fails.append(f"fisher({xT},{xN}): prod {fp} vs ref {row['fisher_p']}")
        cpT = stats.clopper_pearson(xT, 10)
        if not (approx(cpT[0], row["cpT"][0]) and approx(cpT[1], row["cpT"][1])):
            fails.append(f"cpT({xT}): prod {cpT} vs ref {row['cpT']}")
        cpN = stats.clopper_pearson(xN, 10)
        if not (approx(cpN[0], row["cpN"][0]) and approx(cpN[1], row["cpN"][1])):
            fails.append(f"cpN({xN}): prod {cpN} vs ref {row['cpN']}")
        dci = stats.newcombe_diff_ci(xT, xN)
        if not (approx(dci[0], row["diff_ci"][0]) and approx(dci[1], row["diff_ci"][1])):
            fails.append(f"diff_ci({xT},{xN}): prod {dci} vs ref {row['diff_ci']}")

    # 2) production matches externally-derivable closed-form anchors directly
    a2 = 0.025
    n = 10
    # CP extremes
    if not approx(stats.clopper_pearson(0, n)[1], 1 - a2 ** (1 / n)):
        fails.append("anchor CP0.upper")
    if not approx(stats.clopper_pearson(10, n)[0], a2 ** (1 / n)):
        fails.append("anchor CP10.lower")
    if not approx(stats.clopper_pearson(1, n)[0], 1 - (1 - a2) ** (1 / n)):
        fails.append("anchor CP1.lower")
    # Fisher separation and central
    if not approx(stats.fisher_exact_two_sided(10, 0), 2.0 / comb(20, 10)):
        fails.append("anchor Fisher.sep")
    if not approx(stats.fisher_exact_two_sided(5, 5), 1.0):
        fails.append("anchor Fisher.central")

    # 3) structural properties across all pairs
    for xT in range(11):
        for xN in range(11):
            fp = stats.fisher_exact_two_sided(xT, xN)
            if not (0.0 - 1e-12 <= fp <= 1.0 + 1e-12):
                fails.append(f"fisher out of [0,1] at ({xT},{xN}): {fp}")
            # symmetry: fisher(xT,xN) == fisher(xN,xT)
            if not approx(fp, stats.fisher_exact_two_sided(xN, xT)):
                fails.append(f"fisher asymmetric ({xT},{xN})")
            lo, hi = stats.newcombe_diff_ci(xT, xN)
            if lo > hi + 1e-12:
                fails.append(f"newcombe inverted at ({xT},{xN})")
            # point estimate contained
            pt = xT / 10 - xN / 10
            if not (lo - 1e-9 <= pt <= hi + 1e-9):
                fails.append(f"newcombe does not contain point est at ({xT},{xN})")
            # equal arms -> symmetric about 0
            if xT == xN and not approx(lo + hi, 0.0):
                fails.append(f"newcombe not symmetric at ({xT},{xN})")
        # CP containment of the point estimate
        cp = stats.clopper_pearson(xT, 10)
        if not (cp[0] - 1e-9 <= xT / 10 <= cp[1] + 1e-9):
            fails.append(f"CP does not contain point est at {xT}")

    if fails:
        print("STATS TEST FAILED (%d):" % len(fails))
        for f in fails[:30]:
            print("  ", f)
        sys.exit(1)
    print("STATS TEST PASSED: production stats.py matches the independent 121-pair "
          "reference table + closed-form anchors + structural properties.")


if __name__ == "__main__":
    main()
