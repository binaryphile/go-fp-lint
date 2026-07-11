// Package impurereach detects functions that transitively reach a
// direct-impure-call site (per impuresource's allowlist) via one or more
// intermediate function calls within the analyzed package — Normand's
// "actions are infectious." It also flags call sites whose callee cannot
// be statically resolved (function-value indirection, closures passed by
// value, interface-mediated dispatch) with a separate, honestly-worded
// "purity cannot be determined" diagnostic, rather than silently
// under-reporting or over-claiming impurity through them. See
// docs/design.md for the full scope boundary — intra-package only, no
// cross-package analysis.Fact propagation this cycle.
package impurereach

import (
	"go/types"
	"sort"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/ssa"

	"github.com/binaryphile/go-fp-lint/impuresource"
)

// Analyzer reports functions that transitively reach a direct-impure-call
// site, and call sites that cannot be statically resolved (see
// functional-programming-unified-guide.md).
var Analyzer = &analysis.Analyzer{
	Name:     "impurereach",
	Doc:      "reports functions that transitively reach a direct-impure-call site, and calls that cannot be statically resolved (see functional-programming-unified-guide.md)",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInfo := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	// static.CallGraph's "start from package-level symbols" loop walks
	// members of every package in prog.AllPackages(), including imports —
	// but buildssa only ever calls Build() on the analyzed package
	// (imports are created via CreatePackage(p, nil, nil, true), files=nil,
	// never built), so imported functions have no Blocks and contribute
	// zero Out edges. The intra-package boundary is enforced by "imports
	// are never built" (verified by fixture #8), not by restricting the
	// graph itself to one package's nodes.
	cg := static.CallGraph(ssaInfo.Pkg.Prog)

	reportTransitive(pass, cg)
	reportIndeterminate(pass, ssaInfo)

	return nil, nil
}

// impureSeedName reports whether fn resolves to a *types.Func matching
// impuresource.ImpureFuncs, returning "pkg.Func" for the diagnostic
// message. Same Recv() == nil guard as impuresource's matchImpureCall —
// reused via the exported allowlist, not reimplemented.
func impureSeedName(fn *ssa.Function) (string, bool) {
	obj, ok := fn.Object().(*types.Func)
	if !ok || obj.Pkg() == nil {
		return "", false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Recv() != nil {
		return "", false
	}
	if !impuresource.ImpureFuncs[obj.Pkg().Path()][obj.Name()] {
		return "", false
	}
	return obj.Pkg().Name() + "." + obj.Name(), true
}

// reportTransitive flags named source functions that reach an impure seed
// via at least one intermediate hop (depth >= 2). Depth-1 nodes are
// direct callers of the seed — already reported by impuresource's
// direct-call diagnostic, so reporting them again here would be duplicate
// and inaccurately worded ("transitively" when it's actually direct).
func reportTransitive(pass *analysis.Pass, cg *callgraph.Graph) {
	type seed struct {
		node *callgraph.Node
		name string
	}
	var seeds []seed
	for _, n := range cg.Nodes {
		if n.Func == nil {
			continue
		}
		if name, ok := impureSeedName(n.Func); ok {
			seeds = append(seeds, seed{node: n, name: name})
		}
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].name < seeds[j].name })

	reachedBy := map[*callgraph.Node]string{}

	for _, s := range seeds {
		type qEntry struct {
			node  *callgraph.Node
			depth int
		}
		visited := map[*callgraph.Node]bool{s.node: true}
		queue := []qEntry{{s.node, 0}}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, e := range cur.node.In {
				caller := e.Caller
				if visited[caller] {
					continue
				}
				visited[caller] = true
				depth := cur.depth + 1
				queue = append(queue, qEntry{caller, depth})
				if depth < 2 {
					continue
				}
				if existing, ok := reachedBy[caller]; !ok || s.name < existing {
					reachedBy[caller] = s.name
				}
			}
		}
	}

	var nodes []*callgraph.Node
	for n := range reachedBy {
		if n.Func == nil || n.Func.Object() == nil {
			continue // anonymous closures are not report targets (see docs/design.md)
		}
		if n.Func.Synthetic != "" {
			continue // General policy, not just a patch for one category:
			// NO synthetic ssa.Function is code a user wrote, so none of
			// them should ever be a report target regardless of why
			// go/ssa generated it. Confirmed against two distinct
			// categories: pointer-receiver wrappers ("from type
			// information", via methodsOf()'s (*T).M calling through to
			// T.M — fixture #10) and bound-method-value closures ("bound
			// method wrapper for ...", via MakeClosure+StaticCallee on a
			// `f := h.Leaf` value — fixture #12). Both still correctly
			// propagate reachability THROUGH the synthetic hop to the
			// real named caller; only the synthetic node itself is
			// excluded from being reported.
		}
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Func.Pos() < nodes[j].Func.Pos() })

	for _, n := range nodes {
		pass.Report(analysis.Diagnostic{
			Pos:     n.Func.Pos(),
			Message: "func " + n.Func.Name() + " transitively calls " + reachedBy[n] + " — actions are infectious (see functional-programming-unified-guide.md)",
		})
	}
}

// reportIndeterminate flags call sites whose callee cannot be resolved
// statically — function-value/stored-closure calls and interface-mediated
// (invoke-mode) calls. These do not feed the transitive-seed set; they
// only mark the function containing them.
func reportIndeterminate(pass *analysis.Pass, ssaInfo *buildssa.SSA) {
	for _, fn := range ssaInfo.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				site, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := site.Common()
				if common.StaticCallee() != nil {
					continue
				}
				// Excludes calls confirmed to lower to *ssa.Builtin
				// (verified empirically for println — fixture #11); not
				// a verified-exhaustive list of every builtin's SSA
				// lowering shape. A missed builtin shape would at worst
				// be a false-negative "indeterminate" report on an
				// already-out-of-scope, non-purity-relevant category.
				if _, isBuiltin := common.Value.(*ssa.Builtin); isBuiltin {
					continue
				}
				msg := "call via function value — purity cannot be determined (see functional-programming-unified-guide.md)"
				if common.IsInvoke() {
					msg = "interface-dispatched call — purity cannot be determined (see functional-programming-unified-guide.md)"
				}
				pass.Report(analysis.Diagnostic{
					Pos:     site.Pos(),
					Message: msg,
				})
			}
		}
	}
}
