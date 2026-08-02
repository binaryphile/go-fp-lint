// Command contractcheck-vet runs contractcheck as a standalone go vet
// -vettool driver (singlechecker), so score.sh can apply the structural
// contract check to a real candidate package instead of an analysistest
// fixture set.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/binaryphile/go-fp-lint/experiments/coachingmin87588/contractcheck"
)

func main() {
	singlechecker.Main(contractcheck.Analyzer)
}
