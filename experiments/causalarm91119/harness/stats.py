#!/usr/bin/env python3
"""Exact statistics for the #93569 causal arm (prereg §7/§8). Pure-Python, no
scipy/statsmodels (absent on this box). n=10/arm makes exact binomial and
hypergeometric computation trivial via math.comb (full integer precision; no
numeric approximation in the tail sums).

Production implementation. Verified against an independently-generated frozen
reference table (harness/stats_refs.json) whose provenance is a DIFFERENT
numeric path plus externally-published anchor values (see gen_stats_refs.py).

Frozen conventions (harness/analysis-config.json):
  alpha = 0.05, ci = 0.95 (two-sided)
  Fisher: two-sided = sum of all fixed-margin tables with hypergeometric
          probability <= that of the observed table (NOT double-the-tail).
  Per-arm CI: Clopper-Pearson (exact binomial).
  Difference CI (p_T - p_N): Newcombe hybrid-score (method 10).
"""

from math import comb, sqrt

# Frozen z for a two-sided 95% interval (Newcombe/Wilson). Standard-normal
# 0.975 quantile to 7 places.
Z_975 = 1.959964


def binom_pmf(k: int, n: int, p: float) -> float:
    """Exact binomial pmf C(n,k) p^k (1-p)^(n-k)."""
    if k < 0 or k > n:
        return 0.0
    return comb(n, k) * (p ** k) * ((1.0 - p) ** (n - k))


def _binom_upper_tail(x: int, n: int, p: float) -> float:
    """P(X >= x) for X~Binom(n,p)."""
    return sum(binom_pmf(k, n, p) for k in range(x, n + 1))


def _binom_lower_tail(x: int, n: int, p: float) -> float:
    """P(X <= x) for X~Binom(n,p)."""
    return sum(binom_pmf(k, n, p) for k in range(0, x + 1))


def clopper_pearson(x: int, n: int, alpha: float = 0.05):
    """Exact Clopper-Pearson two-sided (1-alpha) interval for a binomial rate.

    Defined by the binomial tails (Clopper & Pearson 1934):
      lower p_L solves  P(X >= x | p_L) = alpha/2   (0 when x == 0)
      upper p_U solves  P(X <= x | p_U) = alpha/2   (1 when x == n)
    Solved by bisection on the exact tail sums (monotone in p). Boundaries at
    x==0 and x==n handled explicitly.
    """
    if not (0 <= x <= n):
        raise ValueError("x out of range")
    a2 = alpha / 2.0
    # lower bound
    if x == 0:
        lo = 0.0
    else:
        blo, bhi = 0.0, 1.0
        for _ in range(200):
            mid = (blo + bhi) / 2.0
            # upper tail P(X>=x) increases with p; find p where it == a2
            if _binom_upper_tail(x, n, mid) < a2:
                blo = mid
            else:
                bhi = mid
        lo = (blo + bhi) / 2.0
    # upper bound
    if x == n:
        hi = 1.0
    else:
        blo, bhi = 0.0, 1.0
        for _ in range(200):
            mid = (blo + bhi) / 2.0
            # lower tail P(X<=x) decreases with p; find p where it == a2
            if _binom_lower_tail(x, n, mid) > a2:
                blo = mid
            else:
                bhi = mid
        hi = (blo + bhi) / 2.0
    return (lo, hi)


def _hypergeom_pmf(a: int, r1: int, r2: int, c1: int) -> float:
    """Prob of the 2x2 table with cell a, row sums r1,r2, col1 sum c1 under the
    hypergeometric (fixed-margins) null. Cells: [[a, r1-a],[c1-a, r2-(c1-a)]].
    Exact via math.comb integer arithmetic then a single division.
    """
    n = r1 + r2
    b = c1 - a
    if a < 0 or b < 0 or (r1 - a) < 0 or (r2 - b) < 0:
        return 0.0
    return comb(r1, a) * comb(r2, b) / comb(n, c1)


def fisher_exact_two_sided(xT: int, xN: int, n_per_arm: int = 10) -> float:
    """Two-sided Fisher's exact p for the 2x2 [[xT, n-xT],[xN, n-xN]].

    Margins fixed: row sums = n each; col1 sum = xT+xN. Two-sided p = sum of the
    probabilities of ALL tables (same margins) whose probability is <= that of
    the observed table (Fisher-Irwin / point-probability method), with a small
    tie tolerance so floating equal-probability tables are included.
    """
    r1 = n_per_arm
    r2 = n_per_arm
    c1 = xT + xN
    p_obs = _hypergeom_pmf(xT, r1, r2, c1)
    tol = 1e-9
    total = 0.0
    a_lo = max(0, c1 - r2)
    a_hi = min(r1, c1)
    for a in range(a_lo, a_hi + 1):
        pa = _hypergeom_pmf(a, r1, r2, c1)
        if pa <= p_obs * (1.0 + tol):
            total += pa
    return min(1.0, total)


def wilson_interval(x: int, n: int, z: float = Z_975):
    """Wilson score interval for a binomial rate (used by Newcombe)."""
    p = x / n
    z2 = z * z
    denom = 1.0 + z2 / n
    center = (p + z2 / (2 * n)) / denom
    half = (z * sqrt(p * (1 - p) / n + z2 / (4 * n * n))) / denom
    return (center - half, center + half)


def newcombe_diff_ci(xT: int, xN: int, nT: int = 10, nN: int = 10, z: float = Z_975):
    """Newcombe hybrid-score CI for p_T - p_N (method 10; Newcombe 1998).

    With Wilson intervals (l1,u1) for arm T and (l2,u2) for arm N:
      lower = (pT - pN) - sqrt((pT - l1)^2 + (u2 - pN)^2)
      upper = (pT - pN) + sqrt((u1 - pT)^2 + (pN - l2)^2)
    Arm order preserved (T minus N).
    """
    pT = xT / nT
    pN = xN / nN
    l1, u1 = wilson_interval(xT, nT, z)
    l2, u2 = wilson_interval(xN, nN, z)
    diff = pT - pN
    lower = diff - sqrt((pT - l1) ** 2 + (u2 - pN) ** 2)
    upper = diff + sqrt((u1 - pT) ** 2 + (pN - l2) ** 2)
    return (lower, upper)


if __name__ == "__main__":
    # quick smoke
    print("CP 3/10:", clopper_pearson(3, 10))
    print("Fisher [[3,7],[1,9]]:", fisher_exact_two_sided(3, 1))
    print("Newcombe 3T vs 1N:", newcombe_diff_ci(3, 1))
