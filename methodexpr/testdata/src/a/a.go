package a

import "github.com/binaryphile/fluentfp/slice"

// Developer has a value-receiver IsIdle method — rewritable to Developer.IsIdle.
type Developer struct{ idle bool }

func (d Developer) IsIdle() bool { return d.idle }

// User has a POINTER-receiver IsActive method — (*User).IsActive has type
// func(*User) bool, never a drop-in for func(User) bool.
type User struct{ active bool }

func (u *User) IsActive() bool { return u.active }

// Widget's HasPrefix takes an argument — a call site passing one is not a
// plain passthrough.
type Widget struct{ name string }

func (w Widget) HasPrefix(s string) bool { return false }

// Thing's Identify returns a concrete string, assignable-but-not-identical to
// the `any` return type a lambda might declare.
type Thing struct{}

func (t Thing) Identify() string { return "" }

var globalDev Developer

// --- positive: value-receiver, plain passthrough, identical result type ---

func PosValueReceiver(xs []Developer) slice.Mapper[Developer] {
	return slice.From(xs).KeepIf(func(d Developer) bool { return d.IsIdle() }) // want `inline lambda can be a method expression: replace with Developer.IsIdle`
}

// --- negatives (silent) ---

// Pointer receiver — (*User).IsActive is never a drop-in for func(User) bool.
func NegPointerReceiver(xs []User) slice.Mapper[User] {
	return slice.From(xs).KeepIf(func(u User) bool { return u.IsActive() })
}

// Already a named function, not an inline lambda — methodexpr only inspects
// *ast.FuncLit arguments.
func namedPred(d Developer) bool { return d.IsIdle() }

func NegNamedFunc(xs []Developer) slice.Mapper[Developer] {
	return slice.From(xs).KeepIf(namedPred)
}

// Extra call arg — w.HasPrefix("x") is not a plain passthrough of the lambda's
// own sole parameter.
func NegExtraArgs(xs []Widget) slice.Mapper[Widget] {
	return slice.From(xs).KeepIf(func(w Widget) bool { return w.HasPrefix("x") })
}

// Multi-statement body — not a single return-passthrough statement.
func NegMultiStatement(xs []Developer) slice.Mapper[Developer] {
	return slice.From(xs).KeepIf(func(d Developer) bool {
		ok := d.IsIdle()
		return ok
	})
}

// Receiver of the inner call is NOT the lambda's own parameter (captures an
// outer variable instead) — not a passthrough of d.
func NegWrongReceiver(xs []Developer) slice.Mapper[Developer] {
	return slice.From(xs).KeepIf(func(d Developer) bool { return globalDev.IsIdle() })
}

// Result-type mismatch: Identify() returns a concrete string, assignable to
// but not IDENTICAL to the lambda's declared `any` return type — substituting
// Thing.Identify (type func(Thing) string) would not match Convert's expected
// func(Thing) any.
func NegResultTypeMismatch(xs []Thing) slice.Mapper[any] {
	return slice.From(xs).Convert(func(t Thing) any { return t.Identify() })
}
