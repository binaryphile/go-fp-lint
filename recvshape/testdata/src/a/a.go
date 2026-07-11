package a

import "sync"

// --- positives: pointer receiver, never mutates, no exclusion applies ---

// NoMutate: single pointer-receiver method, pure accessor.
type NoMutate struct{ x int }

func (n *NoMutate) Get() int { return n.x } // want "Get has a pointer receiver on NoMutate but never mutates"

// BlankRecv: blank/unnamed receiver has no identifier to trace mutation
// through, so it's always eligible to flag (subject to exclusions).
type BlankRecv struct{ x int }

func (*BlankRecv) Noop() {} // want "Noop has a pointer receiver on BlankRecv but never mutates"

// --- negatives: legitimate mutation ---

// Mutates: direct field assignment through the receiver — legitimate
// pointer receiver, not flagged.
type Mutates struct{ x int }

func (m *Mutates) Set(x int) { m.x = x }

// Mixed: one method mutates (legitimate pointer receiver), the other is a
// pure accessor — type-level consistency exemption means BOTH stay silent,
// not just the mutator.
type Mixed struct{ x int }

func (m *Mixed) Set(x int) { m.x = x }
func (m *Mixed) Get() int  { return m.x }

// IncDecMutates: IncDecStmt on a receiver field counts as mutation.
type IncDecMutates struct{ n int }

func (c *IncDecMutates) Inc() { c.n++ }

// WholeValueMutates: `*t = ...` whole-value pointer-deref assignment
// counts as mutation.
type WholeValueMutates struct{ x int }

func (w *WholeValueMutates) Reset() { *w = WholeValueMutates{} }

// AddrOf: `&t.field` address-of a receiver field, conservative mutation
// heuristic — favors false-negative over false-positive, not flagged even
// though this particular body doesn't itself write through the pointer.
type AddrOf struct{ x int }

func (a *AddrOf) Ptr() *int { return &a.x }

// --- negatives: lock exclusion (ported lockPath) ---

type innerLock struct {
	mu sync.Mutex
}

// LockDirect: direct sync.Mutex field — pointer receiver required,
// never flagged regardless of mutation.
type LockDirect struct {
	mu sync.Mutex
	x  int
}

func (l *LockDirect) Get() int { return l.x }

// LockNested: mutex reached through a named (non-embedded) struct field,
// two struct layers deep (LockNested -> innerLock -> sync.Mutex).
type LockNested struct {
	inner innerLock
}

func (l *LockNested) Noop() {}

// LockEmbedded: mutex reached through true Go embedding (unqualified
// field). Structurally identical to LockNested for lockPath's field
// iteration (embedded vs named fields are both just struct fields), kept
// as a distinct fixture for the embedding syntax per the plan's fixture
// matrix.
type LockEmbedded struct {
	innerLock
}

func (l *LockEmbedded) Noop() {}

// ArrayLock: mutex reached through an array-of-structs field — lockPath
// unwraps array element types before checking, same as upstream copylock.
type ArrayLock struct {
	locks [2]innerLock
}

func (a *ArrayLock) Noop() {}

// PtrLockHolder: mutex reached ONLY through a *pointer* field to a
// lock-containing struct. This is a POSITIVE case, not an exclusion:
// upstream copylock's own lockPath explicitly treats pointers (and
// interfaces) as "safe to copy" — copying a struct that holds a *pointer*
// to a mutex does not duplicate the mutex, only the pointer. So the
// ported lockPath correctly does NOT exclude this type, and a
// non-mutating pointer-receiver method on it is legitimately flaggable —
// mirroring copylocks' own behavior exactly rather than being a gap.
type PtrLockHolder struct {
	*innerLock
}

func (p *PtrLockHolder) Noop() {} // want "Noop has a pointer receiver on PtrLockHolder but never mutates"

// --- negative: interface exclusion ---

type Stringer interface {
	String() string
}

// InterfaceType: *T satisfies Stringer, lexically spelled out below via
// the `var _ Stringer = ...` assertion — pointer receiver required for
// interface-satisfaction consistency, never flagged.
type InterfaceType struct{ x int }

func (i *InterfaceType) String() string { return "" }

var _ Stringer = (*InterfaceType)(nil)

// --- negative: composition — two simultaneous exclusion conditions ---

// LockAndInterface: both lock-containing AND satisfies an in-package
// interface via *T. Verifies exclusion ordering never double-flags or
// crashes when multiple exclusion conditions independently apply to the
// same type.
type LockAndInterface struct {
	mu sync.Mutex
	x  int
}

func (l *LockAndInterface) String() string { return "" }

var _ Stringer = (*LockAndInterface)(nil)

// --- not applicable: no pointer-receiver methods at all ---

// ValueOnly: value receiver only — nothing for recvshape to examine.
type ValueOnly struct{ x int }

func (v ValueOnly) Get() int { return v.x }
