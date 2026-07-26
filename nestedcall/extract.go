// change_me-placeholder extraction fix (jeeves #66034): the Tier-B codemod
// half of nestedcall's paren-depth and uniform-commas diagnostics. Both
// violations share the same remedy in fluentfp-guide.md / go-development-
// guide.md prose -- "extract the inner call into an intermediate named
// variable" -- so one shared fix builder serves both diagnostics.
//
// Safety domain (arrived at over four adversarial /grade rounds -- see
// docs/design.md §v12 for the full history): a naive "hoist the nested call
// above the statement" transform is unsound in general (evaluation order,
// scope, goto, typing, comments). Rather than build a general
// evaluation-order/purity analysis, the fix narrows to a small domain where
// none of those hazards can arise BY CONSTRUCTION:
//
//  1. the enclosing statement is `return <outerCall>` (single result) or a
//     bare `*ast.ExprStmt` whose sole expression is `<outerCall>`;
//  2. the candidate is exactly `outerCall.Args[0]`;
//  3. `outerCall.Fun` is a bare `*ast.Ident` resolving to a package-level
//     func (no receiver/function-operand evaluation to reorder);
//  4. the candidate has exactly one, typed result (no tuples, no untyped
//     constants defaulting under a `:=`);
//  5. no `goto` anywhere in the nearest enclosing FuncDecl/FuncLit;
//  6. no comment intersects the candidate's own span.
//
// Any call not meeting every condition gets no fix -- diagnostic only.
package nestedcall

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// parentIndex maps each AST node to its immediate parent, built once per
// file via a single stack-based ast.Inspect walk.
type parentIndex struct {
	parent map[ast.Node]ast.Node
}

func buildParentIndex(file *ast.File) *parentIndex {
	idx := &parentIndex{parent: make(map[ast.Node]ast.Node)}
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		if len(stack) > 0 {
			idx.parent[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return idx
}

// changeMePattern matches existing change_me_N placeholder identifiers so a
// fresh allocator can seed past whatever a file already contains.
var changeMePattern = regexp.MustCompile(`^change_me_(\d+)$`)

// placeholderAllocator hands out change_me_N names, unique within one file.
// Each call to allocate reserves its name immediately in the allocator's own
// state (not just in the unchanged source AST), so two independently-fixed
// violations in the same file can never both compute the same name.
type placeholderAllocator struct {
	next int
}

func newPlaceholderAllocator(file *ast.File) *placeholderAllocator {
	maxSeen := 0
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		m := changeMePattern.FindStringSubmatch(id.Name)
		if m == nil {
			return true
		}
		if v, err := strconv.Atoi(m[1]); err == nil && v > maxSeen {
			maxSeen = v
		}
		return true
	})
	return &placeholderAllocator{next: maxSeen + 1}
}

func (a *placeholderAllocator) allocate() string {
	n := a.next
	a.next++
	return "change_me_" + strconv.Itoa(n)
}

// extractionFix builds a SuggestedFix for outerCall's candidate argument, or
// returns ok=false if any safety condition fails.
func extractionFix(pass *analysis.Pass, file *ast.File, idx *parentIndex, alloc *placeholderAllocator, outerCall, candidate *ast.CallExpr) (analysis.SuggestedFix, bool) {
	stmt, ok := enclosingEligibleStmt(idx, outerCall)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	if !isDirectBlockMember(idx, stmt) {
		return analysis.SuggestedFix{}, false
	}
	if len(outerCall.Args) == 0 || outerCall.Args[0] != candidate {
		return analysis.SuggestedFix{}, false
	}
	if !isPackageLevelIdentCallee(pass, outerCall) {
		return analysis.SuggestedFix{}, false
	}
	if !hasSingleTypedResult(pass, candidate) {
		return analysis.SuggestedFix{}, false
	}
	if enclosingFuncHasGoto(idx, outerCall) {
		return analysis.SuggestedFix{}, false
	}
	if commentIntersects(file, candidate) {
		return analysis.SuggestedFix{}, false
	}

	indent, ok := lineIndent(pass, stmt)
	if !ok {
		return analysis.SuggestedFix{}, false
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, pass.Fset, candidate); err != nil {
		return analysis.SuggestedFix{}, false
	}

	name := alloc.allocate()
	insertText := name + " := " + buf.String() + "\n" + indent

	return analysis.SuggestedFix{
		Message: "extract to " + name,
		TextEdits: []analysis.TextEdit{
			{Pos: stmt.Pos(), End: stmt.Pos(), NewText: []byte(insertText)},
			{Pos: candidate.Pos(), End: candidate.End(), NewText: []byte(name)},
		},
	}, true
}

// enclosingEligibleStmt reports the statement directly containing outerCall
// when that statement is `return outerCall` (single result) or a bare
// `outerCall`-only ExprStmt -- the two shapes where the outer call IS the
// entire statement, so there is no sibling operand to reorder against.
func enclosingEligibleStmt(idx *parentIndex, outerCall *ast.CallExpr) (ast.Stmt, bool) {
	switch p := idx.parent[outerCall].(type) {
	case *ast.ReturnStmt:
		if len(p.Results) == 1 && p.Results[0] == ast.Expr(outerCall) {
			return p, true
		}
	case *ast.ExprStmt:
		if p.X == ast.Expr(outerCall) {
			return p, true
		}
	}
	return nil, false
}

// isDirectBlockMember reports whether stmt is a direct element of a
// *ast.BlockStmt.List -- excludes if/for/switch init clauses (never direct
// list members) and labeled statements (whose parent is *ast.LabeledStmt,
// not *ast.BlockStmt directly), since a goto could jump over the inserted
// declaration into a labeled statement's scope.
func isDirectBlockMember(idx *parentIndex, stmt ast.Stmt) bool {
	_, ok := idx.parent[stmt].(*ast.BlockStmt)
	return ok
}

// isPackageLevelIdentCallee reports whether call.Fun is a bare *ast.Ident
// resolving to a package-level, non-method function -- ruling out
// receiver/function-operand evaluation (method calls, call-returning-call,
// index expressions) entirely.
func isPackageLevelIdentCallee(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil
}

// hasSingleTypedResult reports whether candidate evaluates to exactly one,
// non-constant result -- rejects multi-result calls (a *types.Tuple type)
// and, more broadly, ANY compile-time-constant candidate. A constant call
// result (e.g. complex(1, 2), a builtin applied to constant args) is
// recorded by go/types as converted to its ARGUMENT-POSITION type at the
// original call site -- extracting it into a bare `:=` re-evaluates it with
// no such context, where an untyped constant defaults to a WIDER type (e.g.
// complex128 instead of complex64) that may no longer be assignable back to
// the original parameter. Checking TypeOf's Underlying/IsUntyped bit at the
// candidate's ORIGINAL (already-converted) position can't detect this --
// the untyped-ness is exactly what the conversion consumed. Rejecting every
// constant candidate is a strict superset of "reject untyped constants"
// (an ordinary function call is never constant in Go, so this costs nothing
// on the common path) and needs no re-type-checking in isolation.
func hasSingleTypedResult(pass *analysis.Pass, candidate ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[candidate]
	if !ok || tv.Type == nil {
		return false
	}
	if tv.Value != nil {
		return false
	}
	_, isTuple := tv.Type.(*types.Tuple)
	return !isTuple
}

// enclosingFuncHasGoto reports whether the nearest enclosing FuncDecl or
// FuncLit body containing node has ANY goto statement -- a function-wide,
// conservative scan (not just "is this statement labeled"), since a goto
// earlier in the same function can jump over the newly-inserted declaration
// regardless of whether the modified statement itself carries a label.
// Scoped to the innermost enclosing function only: a goto inside an
// unrelated sibling FuncLit must not suppress a fix in a different,
// goto-free nested closure.
func enclosingFuncHasGoto(idx *parentIndex, node ast.Node) bool {
	var body *ast.BlockStmt
	for n := node; n != nil; n = idx.parent[n] {
		switch f := n.(type) {
		case *ast.FuncDecl:
			body = f.Body
		case *ast.FuncLit:
			body = f.Body
		default:
			continue
		}
		break
	}
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if lit, isLit := n.(*ast.FuncLit); isLit && lit.Body != body {
			return false // don't descend into a nested closure's own goto scope
		}
		if b, ok := n.(*ast.BranchStmt); ok && b.Tok == token.GOTO {
			found = true
			return false
		}
		return true
	})
	return found
}

// commentIntersects reports whether any comment group in file positionally
// intersects candidate's own span -- iterating file.Comments directly (not
// a single ast.CommentMap[candidate] lookup) so a comment attached to a
// DESCENDANT node (e.g. inner(/* why */ x), attached to x) is still caught.
func commentIntersects(file *ast.File, candidate ast.Node) bool {
	for _, g := range file.Comments {
		if g.Pos() < candidate.End() && g.End() > candidate.Pos() {
			return true
		}
	}
	return false
}

// lineIndent returns the source bytes from stmt's enclosing line's start up
// to stmt.Pos(), or ok=false if that span contains anything other than
// spaces/tabs (the statement isn't alone on its line, so there's no safe
// indentation to derive -- e.g. a one-line block or a semicolon-separated
// preceding statement) or the source can't be read/bounds-checked safely.
func lineIndent(pass *analysis.Pass, stmt ast.Stmt) (string, bool) {
	position := pass.Fset.Position(stmt.Pos())
	src, err := readSource(pass, position.Filename)
	if err != nil {
		return "", false
	}
	lineStartOffset := position.Offset - (position.Column - 1)
	if lineStartOffset < 0 || position.Offset > len(src) || lineStartOffset > position.Offset {
		return "", false
	}
	prefix := src[lineStartOffset:position.Offset]
	for _, b := range prefix {
		if b != ' ' && b != '\t' {
			return "", false
		}
	}
	return string(prefix), true
}

// readSource prefers pass.ReadFile (respects analyzer-driver overlays) and
// falls back to os.ReadFile when the running x/tools version's driver
// leaves it nil.
func readSource(pass *analysis.Pass, filename string) ([]byte, error) {
	if pass.ReadFile != nil {
		return pass.ReadFile(filename)
	}
	return os.ReadFile(filename)
}
