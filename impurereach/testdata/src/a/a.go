package a

import (
	"os"
	"time"

	"b"
)

// --- Fixture 1: 2-hop chain ---

// funcB directly calls os.Getenv. It is a DIRECT caller of the seed
// (depth 1) — impuresource already reports this call site; impurereach
// must not duplicate it under "transitively calls" wording, so funcB gets
// no diagnostic here (fixture #1a, negative).
func funcB() {
	os.Getenv("X")
}

// funcA calls funcB — genuinely 2 hops from the seed (fixture #1b).
func funcA() { // want "func funcA transitively calls os.Getenv"
	funcB()
}

// --- Fixture 2: 3-hop chain ---

func funcZ() {
	time.Now()
}

func funcY() { // want "func funcY transitively calls time.Now"
	funcZ()
}

func funcX() { // want "func funcX transitively calls time.Now"
	funcY()
}

// --- Fixture 3: self-recursion + direct call — no diagnostic, no infinite loop ---

// funcRec is both self-recursive and a direct caller of the seed (depth
// 1) — must not be flagged, and BFS must terminate (fixture #3, negative).
func funcRec(n int) {
	if n <= 0 {
		os.Getenv("X")
		return
	}
	funcRec(n - 1)
}

// --- Fixture 4: mutual recursion reaching an impure call ---

func funcO() {
	os.Getenv("Y")
}

// funcN and funcM are mutually recursive; funcN also calls funcO
// (depth 1, excluded). funcN is depth 2, funcM is depth 3 — proves cycle
// safety and correct depth counting (fixture #4).
func funcN() { // want "func funcN transitively calls os.Getenv"
	funcO()
	funcM()
}

func funcM() { // want "func funcM transitively calls os.Getenv"
	funcN()
}

// --- Fixture 5: IIFE — anonymous closure invoked immediately ---

// funcIife's anonymous closure is a direct caller (depth 1, unreported —
// anonymous funcs are never report targets anyway) but the enclosing
// named function is depth 2 via the MakeClosure+immediate-Call static
// edge — "actions are infectious" through an IIFE (fixture #5).
func funcIife() { // want "func funcIife transitively calls os.Getenv"
	func() {
		os.Getenv("Z")
	}()
}

// --- Fixture 6: function-value parameter — dynamic dispatch, no propagation ---

// callFunc invokes its parameter, which go/callgraph/static cannot
// resolve statically (StaticCallee() == nil for a *ssa.Parameter value) —
// gets the indeterminate-call diagnostic, not a transitive one.
func callFunc(f func()) {
	f() // want "call via function value"
}

// funcStore passes an impure-calling closure into callFunc via a static
// call — funcStore itself is NOT flagged (no edge exists from callFunc
// into the closure in the graph at all), proving dynamic dispatch blocks
// propagation rather than silently under- or over-reporting (fixture #6,
// negative on funcStore).
func funcStore() {
	callFunc(func() {
		os.Getenv("W")
	})
}

// --- Fixture 7: interface-mediated call — dynamic dispatch, no propagation ---

type impurer interface {
	DoImpure()
}

type impurerImpl struct{}

// DoImpure directly calls os.Getenv — a direct caller (depth 1) on its
// own, unrelated to how it's invoked below.
func (impurerImpl) DoImpure() {
	os.Getenv("V")
}

// funcInterfaceCall's invoke-mode call cannot be resolved statically —
// gets the interface-specific indeterminate diagnostic (fixture #7).
func funcInterfaceCall(i impurer) {
	i.DoImpure() // want "interface-dispatched call"
}

// funcCallerOfInterfaceCall is NOT flagged — no static edge exists from
// funcInterfaceCall into DoImpure (fixture #7, negative).
func funcCallerOfInterfaceCall() {
	var i impurer = impurerImpl{}
	funcInterfaceCall(i)
}

// --- Fixture 8: cross-package boundary — intra-package-only, stated non-goal ---

// funcCrossPackage calls into package b's DirectImpure, which itself
// directly calls os.Getenv — but b's body is never built in this
// per-package SSA program, so no edge (and no diagnostic) can exist here
// (fixture #8, negative; parallel to impuresource's crossPackageNotFlagged).
func funcCrossPackage() {
	b.DirectImpure()
}

// --- Fixture 9: allowlist miss — time.Since is not in the v1 allowlist ---

func funcAllowlistMissLeaf(when time.Time) {
	time.Since(when)
}

// funcAllowlistMissCaller is NOT flagged — time.Since was never a seed
// (fixture #9, negative).
func funcAllowlistMissCaller(when time.Time) {
	funcAllowlistMissLeaf(when)
}

// --- Fixture 10: method participation ---

type helper struct{}

// Leaf directly calls os.Getenv — a direct caller (depth 1), same as a
// plain function would be.
func (helper) Leaf() {
	os.Getenv("M")
}

// funcCallsMethod calls the method via a concrete (non-interface) value —
// a real static call, so it's depth 2 and flagged (fixture #10).
func funcCallsMethod(h helper) { // want "func funcCallsMethod transitively calls os.Getenv"
	h.Leaf()
}

// --- Fixture 12: bound method value — another synthetic-wrapper category ---

// funcMethodValue takes h.Leaf as a value (not an immediate call) — go/ssa
// synthesizes a "bound method wrapper" closure (Synthetic == "bound method
// wrapper for ...", a different category than the pointer-receiver
// wrapper in fixture #10/#1). Calling it resolves statically via
// StaticCallee's *MakeClosure case, so funcMethodValue IS a real caller of
// the synthetic bound-thunk (which itself statically calls the real
// Leaf), proving the general Synthetic != "" exclusion — not just a
// narrow patch for one wrapper category — correctly lets reachability
// propagate THROUGH the synthetic hop to the real named caller while
// still excluding the synthetic hop itself as a report target.
func funcMethodValue(h helper) { // want "func funcMethodValue transitively calls os.Getenv"
	f := h.Leaf
	f()
}

// --- Fixture 11: builtin call — not an indeterminate call ---

// funcBuiltin calls the println builtin — not a purity question, must not
// produce an indeterminate-call diagnostic (fixture #11, negative).
func funcBuiltin() {
	println("hi")
}
