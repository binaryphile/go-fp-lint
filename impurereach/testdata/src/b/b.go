package b

import "os"

// DirectImpure directly calls an allowlisted impure func — used to prove
// the intra-package-only boundary: package a calling into this function
// must NOT be flagged, since this per-package SSA program never builds b's
// body (fixture #8 in docs/design.md's impurereach section).
func DirectImpure() {
	os.Getenv("X")
}
