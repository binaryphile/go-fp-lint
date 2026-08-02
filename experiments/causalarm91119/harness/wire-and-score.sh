#!/usr/bin/env bash
# wire-and-score.sh — score one delegate fluentmap package against the pinned
# oracle (prereg §8; grade R1-F8/R2-6). Generates an ephemeral scoring module
# that replaces the delegate package + the pinned go-fp-lint oracle, then runs a
# test whose ONLY job is to write an atomic schema-validated result file. The
# caller trusts that file, NOT `go test`'s exit code.
#
# Usage: wire-and-score.sh DELEGATE_PKG_DIR ORACLE_WT RESULT_FILE CAMPAIGN SLOT
#   DELEGATE_PKG_DIR : dir containing the delegate's go.mod (module
#                      example.com/delegate/fluentmap exporting var Analyzer)
#   ORACLE_WT        : pinned oracle worktree root (hash-verified by caller)
#   RESULT_FILE      : absolute path the scorer writes {campaign,slot,pass,mismatches}
#   CAMPAIGN, SLOT   : identity stamped into the result
# Exits 0 if the result file was produced, 1 otherwise (caller re-classifies).

set -euo pipefail
IFS=$'\n'
set -o noglob

main() {
  local delegatePkg=$1 oracleWt=$2 resultFile=$3 campaign=$4 slot=$5

  local goBin=$(set +o noglob; ls -d /nix/store/*-go-1.26.*/bin 2>/dev/null | head -1)
  [[ -n $goBin ]] || { echo 'wire-and-score: no nix-store go found' >&2; return 1; }
  export PATH=$goBin:$PATH

  local oracleTestdata=$oracleWt/experiments/causalarm91119/testdata
  [[ -d $oracleTestdata ]] || { echo "wire-and-score: oracle testdata missing at $oracleTestdata" >&2; return 1; }

  # Fresh scoring module (never reused — avoids test-cache/state ambiguity).
  local scoreDir=$(mktemp -d /tmp/score-XXXXXX)
  trap 'rm -rf "$scoreDir"' RETURN

  cat >$scoreDir/go.mod <<END
module scoring

go 1.26.4

require (
	example.com/delegate/fluentmap v0.0.0
	github.com/binaryphile/go-fp-lint v0.0.0
	golang.org/x/tools v0.48.0
)

replace example.com/delegate/fluentmap => $delegatePkg
replace github.com/binaryphile/go-fp-lint => $oracleWt
replace golang.org/x/tools => golang.org/x/tools v0.48.0
END

  # The scorer test: recover-wrapped so a delegate-analyzer panic becomes a
  # recorded FAIL (grade R2-3), not a crash that leaves no datum. Writes the
  # result atomically (temp + rename). Never asserts — a FAIL is a valid datum.
  cat >$scoreDir/score_test.go <<'END'
package scoring

import (
	"encoding/json"
	"os"
	"testing"

	fluentmap "example.com/delegate/fluentmap"
	oracle "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

type result struct {
	Campaign   string   `json:"campaign"`
	Slot       string   `json:"slot"`
	Pass       bool     `json:"pass"`
	Mismatches []string `json:"mismatches"`
	Reason     string   `json:"reason"`
}

func writeResult(path string, r result) {
	if r.Mismatches == nil {
		r.Mismatches = []string{}
	}
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func TestScore(t *testing.T) {
	path := os.Getenv("RESULT_FILE")
	td := os.Getenv("ORACLE_TESTDATA")
	campaign := os.Getenv("CAMPAIGN")
	slot := os.Getenv("SLOT")
	defer func() {
		if rec := recover(); rec != nil {
			writeResult(path, result{Campaign: campaign, Slot: slot, Pass: false,
				Mismatches: []string{"panic in Score/analyzer"}, Reason: "delegate_panic"})
		}
	}()
	pass, mismatches := oracle.Score(td, fluentmap.Analyzer)
	writeResult(path, result{Campaign: campaign, Slot: slot, Pass: pass,
		Mismatches: mismatches, Reason: "scored"})
}
END

  # Offline, deterministic: pinned x/tools (cached), no proxy. GOMODCACHE is the
  # host seed cache (read-only in effect — GOPROXY=off means no code path can
  # write a new module into it; an unresolvable delegate dep fails the build
  # deterministically, scored FAIL by the classifier). GOCACHE is a SHARED,
  # persistent build-artifact cache across scoring calls — safe to share
  # (content-addressed by input hash, unlike GOMODCACHE's source trees) and
  # cuts a ~60s cold compile to seconds on repeat invocations across the
  # campaign's ~60 scoring/control calls.
  local sharedGocache=$(cd $(dirname $0) && pwd)/.gocache
  rm -f "$resultFile"
  (
    cd $scoreDir
    RESULT_FILE=$resultFile ORACLE_TESTDATA=$oracleTestdata CAMPAIGN=$campaign SLOT=$slot \
    GOFLAGS=-mod=mod GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOCACHE=$sharedGocache \
      go test -count=1 -run TestScore . >$scoreDir/gotest.log 2>&1
  ) || true   # go test rc is NOT the signal; the result file is

  if [[ -f $resultFile ]]; then
    return 0
  fi
  echo "wire-and-score: no result file produced; go test log:" >&2
  cat $scoreDir/gotest.log >&2 || true
  return 1
}

main "$@"
