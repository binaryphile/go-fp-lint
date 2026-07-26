// Package nestedcall detects two related fluentfp call-nesting readability
// violations from fluentfp-guide.md / go-development-guide.md: paren-depth
// (don't open more than two parens without closing) and uniform-commas
// (only one nesting level may have multiple arguments). See docs/design.md
// for the detection rules and known limitations.
package nestedcall

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const (
	parenDepthMessage    = "call nesting depth exceeds 2 — extract to an intermediate named variable (see fluentfp-guide.md paren depth rule)"
	uniformCommasMessage = "commas at multiple nesting levels — extract the inner call to a named variable (see fluentfp-guide.md uniform commas rule)"
)

// Analyzer flags call-expression nesting shapes that violate the paren-depth
// or uniform-commas rules.
var Analyzer = &analysis.Analyzer{
	Name: "nestedcall",
	Doc:  "reports call nesting that violates the paren-depth or uniform-commas rules (see fluentfp-guide.md)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		calls := collectCallExprs(file)
		nestedAsArg := nestedAsArgSet(calls)
		idx := buildParentIndex(file)
		alloc := newPlaceholderAllocator(file)

		for _, call := range calls {
			parenDepthViolation := !nestedAsArg[call] && chainDepth(call) > 2
			uniformCommasViolation := hasUniformCommaViolation(call)
			if !parenDepthViolation && !uniformCommasViolation {
				continue
			}

			// change_me-placeholder SuggestedFix (jeeves #66034): candidate
			// is threaded from the SAME violation computation above, not
			// rescanned, and shared between both diagnostics when they
			// agree on one node -- see extract.go's package doc.
			var fix *analysis.SuggestedFix
			if candidate, ok := sharedCandidate(call, parenDepthViolation, uniformCommasViolation); ok {
				if f, ok := extractionFix(pass, file, idx, alloc, call, candidate); ok {
					fix = &f
				}
			}

			if parenDepthViolation {
				diag := analysis.Diagnostic{Pos: call.Pos(), Message: parenDepthMessage}
				if fix != nil {
					diag.SuggestedFixes = []analysis.SuggestedFix{*fix}
				}
				pass.Report(diag)
			}
			if uniformCommasViolation {
				diag := analysis.Diagnostic{Pos: call.Pos(), Message: uniformCommasMessage}
				// Single-fix-per-call rule: when both diagnostics fire on
				// the same call, only paren-depth carries the fix (attached
				// above) -- attaching an identical SuggestedFix to both
				// risks a double-apply by a fix-consumer that doesn't
				// dedupe by content.
				if fix != nil && !parenDepthViolation {
					diag.SuggestedFixes = []analysis.SuggestedFix{*fix}
				}
				pass.Report(diag)
			}
		}
	}
	return nil, nil
}

// sharedCandidate resolves the single extraction candidate for call given
// which diagnostic(s) fired. When both diagnostics fire, they must identify
// the SAME node or the call is treated as independently ambiguous for both
// (no fix) -- matches filterloop's "no silent transform on ambiguous
// shapes" precedent.
func sharedCandidate(call *ast.CallExpr, parenDepthViolation, uniformCommasViolation bool) (*ast.CallExpr, bool) {
	var parenCand, commaCand *ast.CallExpr
	var parenOk, commaOk bool
	if parenDepthViolation {
		parenCand, parenOk = deepestArgCandidate(call)
	}
	if uniformCommasViolation {
		commaCand, commaOk = uniformCommaCandidate(call)
	}
	switch {
	case parenDepthViolation && uniformCommasViolation:
		if !parenOk || !commaOk || parenCand != commaCand {
			return nil, false
		}
		return parenCand, true
	case parenDepthViolation:
		return parenCand, parenOk
	case uniformCommasViolation:
		return commaCand, commaOk
	default:
		return nil, false
	}
}

// deepestArgCandidate returns the single *ast.CallExpr argument of call with
// the strictly greatest chainDepth among call's direct CallExpr args, or
// ok=false when there is no such arg or two args tie at the maximum depth
// (ambiguous -- no fix).
func deepestArgCandidate(call *ast.CallExpr) (*ast.CallExpr, bool) {
	var best *ast.CallExpr
	bestDepth := -1
	tie := false
	for _, arg := range call.Args {
		argCall, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		d := chainDepth(argCall)
		switch {
		case d > bestDepth:
			best, bestDepth, tie = argCall, d, false
		case d == bestDepth:
			tie = true
		}
	}
	if best == nil || tie {
		return nil, false
	}
	return best, true
}

// uniformCommaCandidate returns the single *ast.CallExpr argument of call
// that is itself a multi-arg call, or ok=false when there is none or more
// than one such arg (ambiguous -- no fix).
func uniformCommaCandidate(call *ast.CallExpr) (*ast.CallExpr, bool) {
	if len(call.Args) <= 1 {
		return nil, false
	}
	var best *ast.CallExpr
	count := 0
	for _, arg := range call.Args {
		if argCall, ok := arg.(*ast.CallExpr); ok && len(argCall.Args) > 1 {
			best = argCall
			count++
		}
	}
	if count != 1 {
		return nil, false
	}
	return best, true
}

// collectCallExprs returns every *ast.CallExpr in file, in AST-walk order.
func collectCallExprs(file *ast.File) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// nestedAsArgSet marks every CallExpr that appears literally inside another
// CallExpr's Args slice. A CallExpr in a receiver/Fun position (method-chain
// or func-returning-func shapes) is deliberately NOT marked — only Args
// nesting counts toward paren-depth, matching the guide's own "OK" example
// of a method chain (results.Sort(...).Take(n)) staying within the limit.
func nestedAsArgSet(calls []*ast.CallExpr) map[*ast.CallExpr]bool {
	nested := make(map[*ast.CallExpr]bool)
	for _, call := range calls {
		for _, arg := range call.Args {
			if argCall, ok := arg.(*ast.CallExpr); ok {
				nested[argCall] = true
			}
		}
	}
	return nested
}

// chainDepth returns the maximum number of simultaneously-open call frames
// rooted at call — 1 for the call itself, plus the deepest nested call
// found among its arguments. Siblings are evaluated independently (max, not
// sum): two 2-deep chains passed as separate arguments to the same call
// still peak at depth 3, not 5, since only one chain is ever open at a time
// when reading left to right.
func chainDepth(call *ast.CallExpr) int {
	depth := 0
	for _, arg := range call.Args {
		if argCall, ok := arg.(*ast.CallExpr); ok {
			if d := chainDepth(argCall); d > depth {
				depth = d
			}
		}
	}
	return depth + 1
}

// hasUniformCommaViolation reports whether call has more than one argument
// AND at least one of those arguments is itself a call with more than one
// argument — commas at two nesting levels. Evaluated per adjacent
// parent/child pair (an intentional v1 scope choice; see docs/design.md).
func hasUniformCommaViolation(call *ast.CallExpr) bool {
	if len(call.Args) <= 1 {
		return false
	}
	for _, arg := range call.Args {
		if argCall, ok := arg.(*ast.CallExpr); ok && len(argCall.Args) > 1 {
			return true
		}
	}
	return false
}
