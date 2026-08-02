#!/usr/bin/env bash
# score.sh is the vehicle-aware merger for jeeves #87588's coaching-minimum
# preregistration: structural contract check -> bin/transform-primary
# --only nestedcall -> functional eligibility gate, combined into one frozen
# JSON record. See ../../docs/design.md and the plan's "Pipeline design"
# section for the full contract. transform-primary itself stays generic;
# this script is where the vehicle-specific knowledge lives.
#
# Usage: score.sh CANDIDATE_DIR
#   CANDIDATE_DIR: a directory containing a go.mod (module
#     example.com/delegate/coachingmin87588) and Go source exporting
#     `type User struct { Name, Email string; Active bool }` and
#     `func SummarizeActiveUsers(users []User) string`, using this
#     vehicle's frozen fluentfp/slice stub (fixtures/fluentfp-stub) chain
#     methods (KeepIf/Map) -- NOT real upstream fluentfp, whose actual API
#     has no plain .Map() (frozen-vehicle rationale documented in
#     fixtures/fluentfp-stub/slice/slice.go).
#
# Emits one JSON record on stdout:
#   {"contract_compliant": bool, "compiled": bool,
#    "post_fix_compiled": bool|null, "functional_pass": bool|null,
#    "pre_count": int|null, "post_count": int|null, "residual": bool|null,
#    "clean": bool}

RepoRoot=''
StubDir=''
ContractcheckVetBin=''
WorkDir=''

main() {
  local candidateDir=$1
  [[ -n $candidateDir ]] || fatal 'usage: score.sh CANDIDATE_DIR' 64

  local repoRoot_
  # shellcheck disable=SC2128 # BASH_SOURCE always has at least one element
  repoRoot_=$(cd $(dirname $BASH_SOURCE)/../.. && pwd)
  RepoRoot=$repoRoot_
  StubDir=$RepoRoot/experiments/coachingmin87588/fixtures/fluentfp-stub

  local workDir_
  workDir_=$(mktemp -d /tmp/score-87588-XXXXXX)
  WorkDir=$workDir_
  trap 'rm -rf "$WorkDir"' RETURN

  buildContractcheckVet

  local copyDir=$WorkDir/candidate
  # shellcheck disable=SC2336 # cp -r's implementation-defined attribute
  # copying is irrelevant here -- we only need file contents duplicated
  # into a scratch dir, never the source's exact permission/attribute bits.
  cp -r $candidateDir $copyDir
  addFluentfpReplace $copyDir/go.mod

  local structBlock_
  structBlock_=$(runStructuralCheck $copyDir)
  local compiled_
  # shellcheck disable=SC9003 # $structBlock_ is multi-line by construction (two key=value lines); unquoted expansion would word-split it across passField's positional args
  compiled_=$(passField compiled "$structBlock_")
  local contractCompliant_
  # shellcheck disable=SC9003 # same multi-line hazard as above
  contractCompliant_=$(passField contract_compliant "$structBlock_")

  if [[ $contractCompliant_ != true ]]; then
    emitRecord "$contractCompliant_" "$compiled_" null null null null null
    return
  fi

  local pipeBlock_
  pipeBlock_=$($RepoRoot/bin/transform-primary --only nestedcall $copyDir)
  local postFixCompiled_
  # shellcheck disable=SC9003 # $pipeBlock_ is a JSON blob; unquoted expansion would word-split it across jq's positional args
  postFixCompiled_=$(echo "$pipeBlock_" | jq -r .post_fix_compiled)
  local preCount_
  # shellcheck disable=SC9003 # same JSON-blob hazard as above
  preCount_=$(echo "$pipeBlock_" | jq -r .pre_count)
  local postCount_
  # shellcheck disable=SC9003 # same JSON-blob hazard as above
  postCount_=$(echo "$pipeBlock_" | jq -r .post_count)
  local residual_
  # shellcheck disable=SC9003 # same JSON-blob hazard as above
  residual_=$(echo "$pipeBlock_" | jq -r .residual)

  if [[ $postFixCompiled_ != true ]]; then
    emitRecord true "$compiled_" "$postFixCompiled_" null "$preCount_" null null
    return
  fi

  local functionalPass_
  functionalPass_=$(runFunctionalCheck $copyDir)

  emitRecord true "$compiled_" "$postFixCompiled_" "$functionalPass_" "$preCount_" "$postCount_" "$residual_"
}

# buildContractcheckVet builds the go vet -vettool driver for contractcheck
# once, reused across the structural-check call below.
buildContractcheckVet() {
  local bin_=$WorkDir/contractcheck-vet
  ( cd $RepoRoot && $go build -o "$bin_" ./experiments/coachingmin87588/cmd/contractcheck-vet ) \
    || fatal 'go build ./experiments/coachingmin87588/cmd/contractcheck-vet failed' 1
  ContractcheckVetBin=$bin_
}

# addFluentfpReplace appends a `replace github.com/binaryphile/fluentfp =>
# <StubDir>` line to `modFile`, freezing the vehicle's chain-method contract
# for this scoring call regardless of the candidate's own require version.
addFluentfpReplace() {
  local modFile=$1
  echo "replace github.com/binaryphile/fluentfp => $StubDir" >> $modFile
}

# runStructuralCheck runs contractcheck (via go vet -vettool) against the
# candidate copy and classifies the result. Reuses the same
# diagnostic-shaped-line-vs-build-failure distinction bin/transform-primary
# already established empirically for go-fp-lint's own multichecker driver
# -- confirmed here for go vet's unitchecker output (Phase 3a, this cycle):
# the contract diagnostic and a build failure are distinguished by the
# specific contract message text, not by exit code or stream alone.
runStructuralCheck() {
  local dir=$1
  local stderr_
  stderr_=$(cd $dir && GOFLAGS=-mod=mod GOPROXY=off $go vet -vettool=$ContractcheckVetBin ./... 2>&1 1>/dev/null) ||:

  if [[ -z $stderr_ ]]; then
    echo 'compiled=true'
    echo 'contract_compliant=true'
    return
  fi
  if [[ $stderr_ == *"must use the project's fluentfp/slice chain methods"* ]]; then
    echo 'compiled=true'
    echo 'contract_compliant=false'
    return
  fi
  echo 'compiled=false'
  echo 'contract_compliant=false'
}

# runFunctionalCheck wires an ephemeral scoring module -- go.mod
# require+replace, same technique as causalarm91119/harness/wire-and-
# score.sh's Analyzer wiring -- importing this package's frozen Score
# oracle plus the candidate's (already nestedcall-fixed) SummarizeActiveUsers
# through an explicit field-mapped adapter, and prints true/false.
runFunctionalCheck() {
  local candidateDir=$1
  local scoreDir=$WorkDir/functional
  mkdir -p $scoreDir

  cat >$scoreDir/go.mod <<END
module scoring

go 1.26.4

require (
	example.com/delegate/coachingmin87588 v0.0.0
	github.com/binaryphile/fluentfp v0.0.0
	github.com/binaryphile/go-fp-lint v0.0.0
	golang.org/x/tools v0.48.0
)

replace example.com/delegate/coachingmin87588 => $candidateDir
replace github.com/binaryphile/fluentfp => $StubDir
replace github.com/binaryphile/go-fp-lint => $RepoRoot
replace golang.org/x/tools => golang.org/x/tools v0.48.0
END

  cat >$scoreDir/score_test.go <<'END'
package scoring

import (
	"os"
	"testing"

	delegate "example.com/delegate/coachingmin87588"
	oracle "github.com/binaryphile/go-fp-lint/experiments/coachingmin87588"
)

// adapt bridges the oracle's frozen User type and the candidate's own User
// type (identical field shape by contract) into the oracle's SummarizeFunc.
func adapt(users []oracle.User) string {
	du := make([]delegate.User, len(users))
	for i, u := range users {
		du[i] = delegate.User{Name: u.Name, Email: u.Email, Active: u.Active}
	}
	return delegate.SummarizeActiveUsers(du)
}

func TestScore(t *testing.T) {
	pass, _ := oracle.Score(adapt)
	if pass {
		os.Stdout.WriteString("functional_pass=true\n")
	} else {
		os.Stdout.WriteString("functional_pass=false\n")
	}
}
END

  local out_
  out_=$(cd $scoreDir && GOFLAGS=-mod=mod GOPROXY=off $go test -count=1 -run TestScore -v . 2>&1) ||:
  if [[ $out_ == *'functional_pass=true'* ]]; then
    echo true
    return
  fi
  echo false
}

# passField extracts `field`'s value from a key=value `block_`. (C)
passField() {
  local field=$1 block_=$2
  echo "$block_" | grep ^$field= | cut -d= -f2
}

# emitRecord prints the frozen JSON schema to stdout. Each arg is the
# literal string true, false, or null.
emitRecord() {
  local contractCompliant=$1 compiled=$2 postFixCompiled=$3 functionalPass=$4 preCount=$5 postCount=$6 residual=$7
  local clean=false
  [[ $contractCompliant == true && $compiled == true && $functionalPass == true && $postCount == 0 ]] && clean=true
  printf '{"contract_compliant": %s, "compiled": %s, "post_fix_compiled": %s, "functional_pass": %s, "pre_count": %s, "post_count": %s, "residual": %s, "clean": %s}\n' \
    $contractCompliant $compiled $postFixCompiled $functionalPass $preCount $postCount $residual $clean
}

fatal() {
  local msg=$1 rc=${2:-1}
  echo $msg >&2
  exit $rc
}

go=${go:-go}

return 2>/dev/null

IFS=$'\n'
set -o noglob
set -uo pipefail

set -e
main "$@"
