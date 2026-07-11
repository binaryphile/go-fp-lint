package a

func f1(x int) int         { return x }
func f2(x, y int) int      { return x + y }
func f3(x, y, z int) int   { return x + y + z }
func g0() int              { return 0 }
func g1(x int) int         { return x }
func g2(x, y int) int      { return x + y }
func h0() int              { return 0 }
func h1(x int) int         { return x }
func fv(xs ...int) int     { return len(xs) }

// funcReturner returns a func — used to test the CallExpr.Fun-is-itself-a-
// CallExpr shape (f()(x)), distinct from method-chain-receiver exclusion.
func funcReturner(x int) func(int) int {
	return func(y int) int { return y }
}

type Chain struct{}

func (c Chain) Sort(x int) Chain { return c }
func (c Chain) Take(x int) Chain { return c }

func desc(x int) int { return x }

var (
	a, b, c, n, sortKey int
	results             Chain
	xs                  []int
)

// parenDepthBad: 3 nested opens without closing (f1 -> g1 -> h1) — exceeds
// the "don't open more than two parens without closing" limit.
func parenDepthBad() int {
	return f1(g1(h1(a))) // want "call nesting depth exceeds 2"
}

// parenDepthOk: exactly 2 opens (f1 -> g1) — at the boundary, not a violation.
func parenDepthOk() int {
	return f1(g1(a))
}

// methodChainNotArgNesting: guide's own "OK: 2 opens" example. Sort's
// receiver-position call (results.Sort(...)) is not an Arg of Take, so it
// does not combine with Take's depth.
func methodChainNotArgNesting() Chain {
	return results.Sort(desc(sortKey)).Take(n)
}

// uniformCommasBad: outer f2 has 2 args, inner g2 also has 2 args — commas
// at both nesting levels.
func uniformCommasBad() int {
	return f2(g2(a, b), c) // want "commas at multiple nesting levels"
}

// uniformCommasOkInnerOnly: outer f1 has 1 arg — commas only at the inner
// level (g2's), not a violation.
func uniformCommasOkInnerOnly() int {
	return f1(g2(a, b))
}

// uniformCommasOkOuterOnly: outer f2 has 2 args, but each is single-arg —
// commas only at the outer level, not a violation.
func uniformCommasOkOuterOnly() int {
	return f2(g1(a), b)
}

// bothViolations: outer f2 (2 args) nests g2 (2 args, itself nesting h1) —
// triggers both paren-depth (f2->g2->h1 = 3 opens) and uniform-commas
// (f2 and g2 both multi-arg).
func bothViolations() int {
	return f2(g2(h1(a), b), c) // want "call nesting depth exceeds 2" "commas at multiple nesting levels"
}

// nestedZeroArgCalls: sibling zero-arg nested calls — neither call is
// multi-arg (uniform-commas requires >1 arg to count), and depth stays at 2.
func nestedZeroArgCalls() int {
	return f2(g0(), h0())
}

// funcReturningFunc: outer call's Fun is itself a CallExpr
// (funcReturner(a)), not an Arg — mirrors the method-chain-receiver
// exclusion but via a plain func-returning-func value instead of a method.
func funcReturningFunc() int {
	return funcReturner(a)(b)
}

// variadicCallNoCrash: ellipsis-spread variadic call — sanity that Args
// counting doesn't misbehave on an Ellipsis argument.
func variadicCallNoCrash() int {
	return fv(xs...)
}

// parenthesizedCallee: Fun is a ParenExpr, not a bare Ident/SelectorExpr —
// sanity that the walk doesn't special-case Fun's shape.
func parenthesizedCallee() int {
	return (f1)(a)
}

// mixedSiblingDepth: three sibling args to f3, only one (g1(h1(a))) nests
// deep enough to matter — confirms max-not-sum aggregation across siblings.
func mixedSiblingDepth() int {
	return f3(g1(h1(a)), b, c) // want "call nesting depth exceeds 2"
}
