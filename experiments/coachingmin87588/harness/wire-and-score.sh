#!/usr/bin/env bash
# wire-and-score.sh — thin wrapper bridging jeeves #87588's score.sh (the
# vehicle-aware structural+functional+transform-primary merger) into the
# forked #91119 dispatch-harness contract (jeeves #96780 design §1, R1 P0
# fix on the infra_void predicate). NOT a fork of #91119's own
# wire-and-score.sh — that script scores directly via `go test`; this one
# only shells out to score.sh and bridges its schema.
#
# Usage: wire-and-score.sh DELEGATE_PKG_DIR ORACLE_WT RESULT_FILE CAMPAIGN SLOT
#   DELEGATE_PKG_DIR : dir containing the delegate's go.mod (module
#                      example.com/delegate/coachingmin87588)
#   ORACLE_WT        : pinned oracle worktree root; ORACLE_WT/experiments/
#                      coachingmin87588/score.sh is invoked directly (it
#                      resolves its own repo root relatively, so running it
#                      from inside the pinned worktree is drop-in-safe)
#   RESULT_FILE      : absolute path the wrapper writes the bridged JSON
#                      record to, atomically (write-temp, rename)
#   CAMPAIGN, SLOT   : identity stamped into the result
# Exits 0 if the result file was produced, 1 otherwise (caller re-classifies).

set -euo pipefail
IFS=$'\n'
set -o noglob

# realGoPath returns the direct nix-store go binary path, bypassing any
# project bin/go nix-wrapper on PATH -- an ambient `go` resolving to that
# wrapper hangs score.sh indefinitely (jeeves #87588 Phase 3a finding, see
# score_test.bash's own copy of this same helper). (C)
realGoPath() (
  set +o noglob
  # shellcheck disable=SC2012 # ls -d against a bin/go suffix is the
  # correct filter here; find -iname alone matches .drv/bootstrap entries.
  ls -d /nix/store/*-go-1.2*/bin/go 2>/dev/null | head -1
)

main() {
  local delegatePkg=$1 oracleWt=$2 resultFile=$3 campaign=$4 slot=$5

  local scoreScript=$oracleWt/experiments/coachingmin87588/score.sh
  [[ -x $scoreScript ]] || { echo 'wire-and-score: score.sh missing or not executable at '$scoreScript >&2; return 1; }

  local realGo_
  realGo_=$(realGoPath)
  [[ -n $realGo_ ]] || { echo 'wire-and-score: no nix-store go toolchain found' >&2; return 1; }
  local realGo=$realGo_

  # env -i + a direct nix-store go, mirroring score_test.bash's own
  # sandboxing -- an ambient PATH `go` (the project's bin/go wrapper) hangs
  # score.sh indefinitely rather than failing fast.
  local raw_
  raw_=$(env -i HOME=$HOME PATH=$(dirname $realGo):/usr/bin:/bin:$HOME/.nix-profile/bin \
    go=$realGo GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 \
    GOCACHE=$HOME/.cache/go-build GOMODCACHE=$HOME/go/pkg/mod GOPATH=$HOME/go \
    $scoreScript $delegatePkg) || { echo 'wire-and-score: score.sh exited nonzero' >&2; return 1; }
  [[ -n $raw_ ]] || { echo 'wire-and-score: score.sh produced no output' >&2; return 1; }

  rm -f $resultFile
  local tmp=$resultFile.tmp
  jq -n \
    --argjson raw "$raw_" \
    --arg campaign $campaign \
    --arg slot $slot \
    '
      # first non-passing dimension, in the frozen check order (jeeves
      # #96780 design §1) -- contract_compliant / compiled / post_fix_compiled
      # / functional_pass / residual. Later fields are only non-null once
      # every earlier gate passed (score.sh short-circuits), so this order
      # never masks an earlier real failure behind a later null.
      def failField:
        if .contract_compliant != true then "contract_compliant"
        elif .compiled != true then "compiled"
        elif .post_fix_compiled == false then "post_fix_compiled"
        elif .functional_pass == false then "functional_pass"
        elif .residual == true then "residual"
        else null
        end;
      $raw
      + {
          campaign: $campaign,
          slot: $slot,
          pass: $raw.clean,
          mismatches: ( ($raw|failField) as $f | if $f then [$f] else [] end ),
          infra_void: ($raw.compiled == true and $raw.post_fix_compiled == false)
        }
    ' >$tmp
  mv $tmp $resultFile
}

main "$@"
