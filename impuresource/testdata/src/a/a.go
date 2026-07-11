package a

import (
	"os"
	"time"

	t "time"

	"b"
)

var globalCount int

type globalConfigType struct {
	Name string
}

var globalConfig globalConfigType

const globalConst = 5

func directCalls() {
	os.Getenv("X") // want "direct call to os.Getenv"
	time.Now()     // want "direct call to time.Now"
	t.Now()        // want "direct call to time.Now"
}

type localTimeType struct{}

func (localTimeType) Now() {}

// methodNotFlagged: a method named Now on a local type is not a direct call
// to time.Now — Recv() != nil excludes it (fixture #10).
func methodNotFlagged() {
	var lt localTimeType
	lt.Now()
}

type fakeOS struct{}

func (fakeOS) Getenv(string) string { return "" }

// shadowedOsNotFlagged: a local var literally named "os" with its own
// Getenv method is not the os package — object resolution, not
// name-matching, excludes it (fixture #11).
func shadowedOsNotFlagged() {
	os := fakeOS{}
	os.Getenv("X")
}

func varTouches() {
	x := globalCount   // want "read of package-scope var globalCount"
	globalCount = 5    // want "write to package-scope var globalCount"
	p := &globalCount  // want "address-of package-scope var globalCount"
	globalCount += 1   // want "compound-assign of package-scope var globalCount"
	globalCount++      // want "compound-assign of package-scope var globalCount"
	_ = x
	_ = p
}

// localVarNotFlagged: a function-local var shadowing the package-level
// globalCount is not package-scope — Parent() != pass.Pkg.Scope() excludes
// it (fixture #12).
func localVarNotFlagged() {
	var globalCount int
	globalCount = 10
	_ = globalCount
}

// crossPackageNotFlagged: touching another package's exported var is out of
// scope for v1 — own-package-only boundary (fixture #13).
func crossPackageNotFlagged() {
	_ = b.ExportedVar
	b.ExportedVar = 5
}

// allowlistMissNotFlagged: time.Since is not in the v1 allowlist — the
// allowlist is intentionally incomplete (fixture #14).
func allowlistMissNotFlagged(when time.Time) {
	time.Since(when)
}

// selectorBoundary: a selector into a package-var struct classifies as
// "read of" the base identifier even when used as a write target —
// classification only inspects the identifier's immediate AST parent, not
// deeper into selector chains (fixture #15; documented limitation, not a
// silent mis-label).
func selectorBoundary() {
	_ = globalConfig.Name   // want "read of package-scope var globalConfig"
	globalConfig.Name = "x" // want "read of package-scope var globalConfig"
}

// constNotFlagged: a package-level const is a *types.Const, not a
// *types.Var — falls out of the type switch with no special-casing
// (fixture #16).
func constNotFlagged() {
	_ = globalConst
}
