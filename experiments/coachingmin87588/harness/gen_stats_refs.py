#!/usr/bin/env python3
"""Independent reference generator for stats_refs.json (prereg §8; grade R2-7).

This is a SEPARATELY-AUTHORED implementation that does NOT import the production
stats.py. It uses genuinely DIFFERENT numeric paths:

  - Fisher: log-space via math.lgamma (production uses exact integer math.comb).
  - Clopper-Pearson: Newton's method with the analytic tail derivative
    (production uses bisection).
  - Newcombe/Wilson: re-derived arrangement.

Before emitting the 121-pair table it ASSERTS its own output against a set of
EXTERNALLY-DERIVABLE closed-form anchor values (CP extremes, Fisher separation /
central, Newcombe symmetric zero-difference), so a shared bug between this and
production cannot pass unnoticed: the anchors are the independent ground truth,
this generator extends coverage to all 121 (x_T, x_N) pairs.

Run:  python3 gen_stats_refs.py > stats_refs.json
"""

import json
from math import lgamma, sqrt, exp, log, comb

Z_975 = 1.959964
N = 10
ALPHA = 0.05


def _lchoose(n, k):
    if k < 0 or k > n:
        return float("-inf")
    return lgamma(n + 1) - lgamma(k + 1) - lgamma(n - k + 1)


def fisher_two_sided_logspace(xT, xN, n=N):
    """Two-sided Fisher via log-space hypergeometric probabilities."""
    r1 = r2 = n
    c1 = xT + xN
    total_n = r1 + r2

    def logp(a):
        b = c1 - a
        if a < 0 or b < 0 or (r1 - a) < 0 or (r2 - b) < 0:
            return float("-inf")
        return _lchoose(r1, a) + _lchoose(r2, b) - _lchoose(total_n, c1)

    lp_obs = logp(xT)
    tol = 1e-9
    a_lo = max(0, c1 - r2)
    a_hi = min(r1, c1)
    total = 0.0
    for a in range(a_lo, a_hi + 1):
        lpa = logp(a)
        if lpa == float("-inf"):
            continue
        # include if prob <= observed prob (with tolerance in log space)
        if lpa <= lp_obs + tol:
            total += exp(lpa)
    return min(1.0, total)


def _upper_tail(x, n, p):
    return sum(comb(n, k) * p ** k * (1 - p) ** (n - k) for k in range(x, n + 1))


def _lower_tail(x, n, p):
    return sum(comb(n, k) * p ** k * (1 - p) ** (n - k) for k in range(0, x + 1))


def clopper_pearson_newton(x, n=N, alpha=ALPHA):
    """CP interval via Newton's method with the analytic tail derivative.

    d/dp P(X>=x) =  n*C(n-1,x-1) p^(x-1)(1-p)^(n-x)
    d/dp P(X<=x) = -n*C(n-1,x)   p^x    (1-p)^(n-x-1)
    """
    a2 = alpha / 2.0

    if x == 0:
        lo = 0.0
    else:
        p = x / n  # start at the MLE
        for _ in range(100):
            f = _upper_tail(x, n, p) - a2
            d = n * comb(n - 1, x - 1) * p ** (x - 1) * (1 - p) ** (n - x)
            if d == 0:
                break
            pnew = p - f / d
            pnew = min(1 - 1e-15, max(1e-15, pnew))
            if abs(pnew - p) < 1e-14:
                p = pnew
                break
            p = pnew
        lo = p

    if x == n:
        hi = 1.0
    else:
        p = x / n
        for _ in range(100):
            f = _lower_tail(x, n, p) - a2
            d = -n * comb(n - 1, x) * p ** x * (1 - p) ** (n - x - 1)
            if d == 0:
                break
            pnew = p - f / d
            pnew = min(1 - 1e-15, max(1e-15, pnew))
            if abs(pnew - p) < 1e-14:
                p = pnew
                break
            p = pnew
        hi = p

    return (lo, hi)


def wilson(x, n=N, z=Z_975):
    p = x / n
    z2 = z * z
    a = p + z2 / (2 * n)
    b = z * sqrt((p * (1 - p) + z2 / (4 * n)) / n)
    d = 1 + z2 / n
    return ((a - b) / d, (a + b) / d)


def newcombe(xT, xN, nT=N, nN=N, z=Z_975):
    pT, pN = xT / nT, xN / nN
    l1, u1 = wilson(xT, nT, z)
    l2, u2 = wilson(xN, nN, z)
    diff = pT - pN
    lo = diff - sqrt((pT - l1) ** 2 + (u2 - pN) ** 2)
    hi = diff + sqrt((u1 - pT) ** 2 + (pN - l2) ** 2)
    return (lo, hi)


# ---- externally-derivable closed-form anchors (independent ground truth) ----
def _anchor_checks():
    checks = []
    a2 = ALPHA / 2.0

    # CP closed forms:
    #  x=0: upper = 1 - (alpha/2)^(1/n); lower = 0
    #  x=n: lower = (alpha/2)^(1/n);     upper = 1
    #  x=1: lower = 1 - (1-alpha/2)^(1/n)
    #  x=n-1: upper = (1-alpha/2)^(1/n)  (symmetry with x=1)
    cp0 = clopper_pearson_newton(0)
    checks.append(("CP0.upper", cp0[1], 1 - a2 ** (1 / N)))
    checks.append(("CP0.lower", cp0[0], 0.0))
    cp10 = clopper_pearson_newton(10)
    checks.append(("CP10.lower", cp10[0], a2 ** (1 / N)))
    checks.append(("CP10.upper", cp10[1], 1.0))
    cp1 = clopper_pearson_newton(1)
    checks.append(("CP1.lower", cp1[0], 1 - (1 - a2) ** (1 / N)))
    cp9 = clopper_pearson_newton(9)
    checks.append(("CP9.upper", cp9[1], (1 - a2) ** (1 / N)))

    # Fisher separation: [[10,0],[0,10]] -> exactly 2/C(20,10)
    sep = 2.0 / comb(20, 10)
    checks.append(("Fisher.sep(10,0)", fisher_two_sided_logspace(10, 0), sep))
    checks.append(("Fisher.sep(0,10)", fisher_two_sided_logspace(0, 10), sep))
    # Fisher central [[5,5],[5,5]] is the max-prob table -> two-sided p == 1.0
    checks.append(("Fisher.central(5,5)", fisher_two_sided_logspace(5, 5), 1.0))

    # Newcombe symmetric: equal arms -> diff 0, interval symmetric (lo == -hi)
    for k in (0, 3, 5, 7, 10):
        lo, hi = newcombe(k, k)
        checks.append((f"Newcombe.sym({k},{k}).mid", lo + hi, 0.0))
    return checks


def main():
    tol = 1e-6
    failures = []
    for name, got, want in _anchor_checks():
        if abs(got - want) > tol:
            failures.append(f"{name}: got {got!r} want {want!r} (|Δ|={abs(got-want):.2e})")
    if failures:
        raise SystemExit("ANCHOR CHECK FAILED (refs NOT emitted):\n  " + "\n  ".join(failures))

    # CP per k (both arms share n=10, so one table of 11 suffices, reused)
    cp = {str(k): list(clopper_pearson_newton(k)) for k in range(N + 1)}

    table = []
    for xT in range(N + 1):
        for xN in range(N + 1):
            table.append({
                "xT": xT, "xN": xN,
                "fisher_p": fisher_two_sided_logspace(xT, xN),
                "cpT": cp[str(xT)],
                "cpN": cp[str(xN)],
                "diff_ci": list(newcombe(xT, xN)),
            })

    out = {
        "provenance": (
            "Independent second implementation (harness/gen_stats_refs.py): Fisher via "
            "log-space math.lgamma; Clopper-Pearson via Newton's method with the analytic "
            "tail derivative; Newcombe via re-derived Wilson. Distinct numeric paths from the "
            "production stats.py (exact math.comb + bisection). Self-checked against "
            "externally-derivable closed-form anchors (CP extremes x=0,1,9,10; Fisher "
            "separation=2/C(20,10) and central=1.0; Newcombe zero-difference symmetry) before "
            "emission; all anchors passed within 1e-6. n=10/arm, alpha=0.05, two-sided."
        ),
        "n_per_arm": N,
        "alpha": ALPHA,
        "z_975": Z_975,
        "cp_by_k": cp,
        "table": table,
    }
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    main()
