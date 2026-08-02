#!/usr/bin/env bash
# run-campaign.sh drives the #93569 causal-arm campaign (prereg §6/§8): a
# pre-flight validation suite (integrity replay, stats, mock-delegate
# dry-run) followed by the 20-slot frozen App-D sequential dispatch. See
# harness/README.md and jeeves investigations/2026-08-01-model-routing-causal-
# arm-preregistration.md for the frozen protocol this mechanically executes.

# shellcheck disable=SC2128 # BASH_SOURCE always has at least one element
Here=$(cd $(dirname $BASH_SOURCE) && pwd)
source $Here/journal.bash
source $Here/scorer-control.bash
source $Here/dispatch.bash

# The frozen App-D vector (prereg Appendix D) — slot i -> arm.
FrozenVector=( T N N N T T N T T N N T N N T N T T T N )

# runCampaign.classify determines a slot's terminal status from the scorer-
# control health (before/after) and the delegate's own score result (grade
# R1-F2, R2-3, corrected): a scorer-control failure at EITHER bracket means
# the scorer/oracle infrastructure itself is suspect for this slot -> HALT.
# Otherwise, a delegate that produced no result file, or one that produced a
# result file with pass=false, is a genuine delivery FAIL (never infra-void)
# — the classifier does NOT distinguish "delegate compile error" from
# "delegate panic" from "wrong analyzer": all are FAIL once the scorer is
# proven healthy on both sides. `resultFileExists` is a THREE-state value —
# "yes" / "no" / "malformed" (grade IMPL-R2-2): "malformed" means the scorer
# process exited and left a file, but it failed the shape check (not valid
# JSON, or missing/mistyped required fields) — that is scorer/harness
# corruption, NOT a delegate failure, and must never be silently folded into
# the same bucket as "delegate produced nothing." Echoes "status reason"
# (two words).
runCampaign.classify() {
  local controlBeforeHealthy=$1 controlAfterHealthy=$2 resultFileExists=$3 resultPass=$4

  if [[ $controlBeforeHealthy != healthy ]]; then
    echo 'infra-void scorer_control_before_failed'
    return
  fi
  if [[ $controlAfterHealthy != healthy ]]; then
    echo 'infra-void scorer_control_after_failed'
    return
  fi
  if [[ $resultFileExists == malformed ]]; then
    echo 'infra-void scorer_result_malformed'
    return
  fi
  if [[ $resultFileExists != yes ]]; then
    echo 'fail no_result_file_scorer_healthy'
    return
  fi
  if [[ $resultPass == true ]]; then
    echo 'pass scored'
  else
    echo 'fail scored_not_pass'
  fi
}

# runCampaign.discoverDelegatePkg searches `root` for exactly one go.mod
# declaring `module example.com/delegate/fluentmap` (prereg §3/§8's frozen
# delivery contract; grade R1-F5 — deterministic, no location hints). Echoes
# the containing directory on success; echoes nothing and returns 1 on
# zero-or-multiple matches (the caller classifies this as delivery FAIL).
runCampaign.discoverDelegatePkg() {
  local root=$1
  local matches=$(grep -rl '^module example\.com/delegate/fluentmap$' $root --include=go.mod 2>/dev/null ||:)
  local -i n=$(wc -l <<<"${matches:-}")
  [[ -z $matches ]] && n=0
  if (( n != 1 )); then
    echo "runCampaign.discoverDelegatePkg: found $n candidate module(s), need exactly 1" >&2
    return 1
  fi
  dirname $matches
}

# runCampaign.processSlot runs ONE slot (`slot`/`arm`) through the shared
# discovery -> scorer-control -> score -> classify -> journal pipeline.
# `delegatePkgDir` and `rawResultJson` are ALREADY PRODUCED by the caller —
# either a live dispatch.Run (real slot) or a canned mock (dry-run
# validation) — so this function's body is IDENTICAL for both (grade
# R2-9's "mock enters downstream and shares the live path's code").
runCampaign.processSlot() {
  local journalDir=$1 slot=$2 arm=$3 delegatePkgDir=$4 rawResultJson=$5
  local oracleWt=$6 refCorrectDir=$7

  local claudeMetrics_=$(python3 $Here/parse_claude_result.py $rawResultJson)

  local controlBefore=$(scorerControl.Check $refCorrectDir $oracleWt $Here)

  local scoreResult=/tmp/score-slot-$(printf '%02d' $slot)-$$.json
  rm -f $scoreResult
  local resultExists=no resultPass=false mismatches='[]'
  bash $Here/wire-and-score.sh $delegatePkgDir $oracleWt $scoreResult $journalDir slot$slot 2>/dev/null ||:
  # Grade IMPL-F8/R2-2/R3-2 fix: the result file is atomically written
  # (write-temp + rename in wire-and-score.sh's writeResult) but was
  # previously consumed via bare `jq -r .pass` with no shape/type check —
  # "schema-validated" was aspirational, not implemented. Validate the
  # required fields exist with the expected types before trusting the
  # file. A file that EXISTS but fails this check is scorer/harness
  # corruption, not a delegate failure — `resultExists=malformed` routes
  # to infra-void via runCampaign.classify, distinct from the genuine
  # "no file at all" FAIL case. Checked UNCONDITIONALLY on wire-and-score.sh's
  # own exit code (R3-2): the check must not depend on that script's exact
  # return contract (currently 0 iff the file exists, but coupling this
  # classification to that implicit invariant is fragile — a file left
  # behind by a partial/nonzero-exit run must still be caught).
  if [[ -f $scoreResult ]]; then
    if jq -e 'type=="object" and (.pass|type=="boolean") and (.mismatches|type=="array")' \
      <$scoreResult >/dev/null 2>&1; then
      resultExists=yes
    else
      resultExists=malformed
    fi
  fi
  if [[ $resultExists == yes ]]; then
    resultPass=$(jq -r .pass <$scoreResult)
    mismatches=$(jq -c .mismatches <$scoreResult)
  fi

  local controlAfter=$(scorerControl.Check $refCorrectDir $oracleWt $Here)

  local classified=$(runCampaign.classify $controlBefore $controlAfter $resultExists $resultPass)
  local status=${classified%% *} reason=${classified#* }

  local combinedMetrics=$(jq -n --argjson c "$claudeMetrics_" --argjson p $resultPass \
    --argjson m "$mismatches" --arg cb $controlBefore --arg ca $controlAfter \
    '$c + {score_pass:$p, mismatches:$m, control_before:$cb, control_after:$ca}')

  # Grade IMPL-R2-4 fix: journal.Record's failure (e.g. the R1-F5
  # already-recorded guard) was previously swallowed — `local x=$(...)` at
  # the call site masks a command substitution's exit code (this is
  # SC2155's exact warning), and the two statements after Record here always
  # succeed, so main() never saw a failure. Propagate it explicitly.
  journal.Record $journalDir $slot $status $reason "$combinedMetrics" || {
    rm -f $scoreResult
    echo "infra-void journal_record_failed"
    return 1
  }
  rm -f $scoreResult

  echo "$status $reason"
}

# runCampaign.preflight runs every HALT-on-fail check the frozen protocol
# requires before slot 1 (prereg §5/§6; plan "Frozen execution semantics"):
# offline env, dojo cwd-persistence mechanism, pinned-oracle clean-tree +
# full-tree hash, stats validation, integrity replay through the real
# scoring path, and a mock-delegate dry-run sharing processSlot's own code.
# Consumes ZERO vector slots and ZERO live claude dispatches. Returns 0 on
# total success, 1 on any failure (with a diagnostic).
runCampaign.preflight() {
  local oracleWt=$1 refCorrectDir=$2 refBrokenDir=$3

  echo 'PRE-FLIGHT 1/7: offline scoring env'
  command -v go >/dev/null || { echo 'FAIL: go not on PATH'; return 1; }
  [[ $(go env GOPROXY) == off ]] || echo 'note: GOPROXY not off in ambient env (wire-and-score.sh sets it per-call)'

  echo 'PRE-FLIGHT 2/7: dojo cwd-persistence mechanism (zero-cost, no claude call)'
  # Anchor: campaign-1 (2026-08-02) discovered empirically that `dojo
  # --project X` does NOT change the wrapped command's cwd -- 5/5 delegates
  # produced real, substantial work that landed outside the persisted
  # directory and was silently discarded on sandbox exit, misclassified as
  # delivery FAIL rather than the infra defect it was. This check would have
  # caught it before any real spend.
  local mechDir=$(mktemp -d /tmp/preflight-mech-XXXXXX)
  dojo --project $mechDir --persist $mechDir --hide $HOME/projects -- \
    bash -c 'cd "$1" && echo probe >probe.txt' _ $mechDir >/dev/null 2>&1
  [[ -f $mechDir/probe.txt ]] && [[ $(cat $mechDir/probe.txt) == probe ]] \
    || { echo "FAIL: a file written by the sandboxed command did not survive to the persisted dir — dojo cwd/persist mechanism broken"; rm -rf $mechDir; return 1; }
  rm -rf $mechDir
  echo 'dojo cwd-persistence mechanism OK'

  echo 'PRE-FLIGHT 3/7: pinned-oracle clean + full-tree hash'
  local dirty=$(git -C $oracleWt status --porcelain)
  [[ -z $dirty ]] || { echo "FAIL: oracle worktree dirty: $dirty"; return 1; }
  local actualHash=$( (cd $oracleWt && find experiments/causalarm91119 -type f | sort | xargs sha256sum | sha256sum | cut -d' ' -f1) )
  local frozenHash=$(jq -r .oracle.full_tree_sha256 <$Here/FROZEN.json)
  [[ $actualHash == $frozenHash ]] || { echo "FAIL: oracle tree hash mismatch: got $actualHash want $frozenHash"; return 1; }
  local oracleHead=$(git -C $oracleWt rev-parse HEAD)
  local frozenOracle=$(jq -r .oracle.commit <$Here/FROZEN.json)
  [[ $oracleHead == $frozenOracle ]] || { echo "FAIL: oracle HEAD mismatch: got $oracleHead want $frozenOracle"; return 1; }

  echo 'PRE-FLIGHT 4/7: stats validated vs independent reference'
  python3 $Here/stats_test.py || { echo 'FAIL: stats_test.py'; return 1; }

  echo 'PRE-FLIGHT 5/7: integrity replay (real scoring path)'
  local rc1=/tmp/preflight-correct-$$.json rc2=/tmp/preflight-broken-$$.json
  rm -f $rc1 $rc2
  bash $Here/wire-and-score.sh $refCorrectDir $oracleWt $rc1 preflight correct ||:
  bash $Here/wire-and-score.sh $refBrokenDir $oracleWt $rc2 preflight broken ||:
  [[ -f $rc1 ]] && [[ $(jq -r .pass <$rc1) == true ]] || { echo 'FAIL: correct reference did not PASS'; return 1; }
  [[ -f $rc2 ]] && [[ $(jq -r .pass <$rc2) == false ]] && [[ $(jq '.mismatches | length' <$rc2) -gt 0 ]] \
    || { echo 'FAIL: broken reference did not FAIL with mismatches'; return 1; }
  rm -f $rc1 $rc2
  echo 'integrity replay OK: correct=PASS broken=FAIL(with mismatches)'

  echo 'PRE-FLIGHT 6/7: brief SHA assertion'
  local nSha=$(sha256sum $Here/briefs/arm-N.txt | cut -d' ' -f1)
  local tSha=$(sha256sum $Here/briefs/arm-T.txt | cut -d' ' -f1)
  local frozenN=$(jq -r .briefs.arm_N_sha256 <$Here/FROZEN.json)
  local frozenT=$(jq -r .briefs.arm_T_sha256 <$Here/FROZEN.json)
  [[ $nSha == $frozenN ]] || { echo "FAIL: arm-N brief SHA drift"; return 1; }
  [[ $tSha == $frozenT ]] || { echo "FAIL: arm-T brief SHA drift"; return 1; }

  echo 'PRE-FLIGHT 7/7: mock-delegate dry-run (shares live processSlot code)'
  local mockJournal=/tmp/preflight-mock-journal-$$
  rm -rf $mockJournal
  journal.Init $mockJournal preflight-mock-vector >/dev/null
  journal.Reserve $mockJournal 1 T
  journal.Dispatch $mockJournal 1 $$
  local mockResult=$(runCampaign.processSlot $mockJournal 1 T $refCorrectDir $Here/fixtures/claude-result-schema.json $oracleWt $refCorrectDir)
  [[ $mockResult == 'pass scored' ]] || { echo "FAIL: mock-delegate dry-run expected 'pass scored', got '$mockResult'"; return 1; }
  rm -rf $mockJournal
  echo 'mock-delegate dry-run OK'

  echo 'PRE-FLIGHT: ALL CHECKS PASSED — no vector slots consumed'
  return 0
}

# runCampaign.cleanupSlot removes ALL per-slot ephemeral artifacts for `slot`
# — the clone, the per-slot module cache, AND the raw claude result files
# (grade IMPL-F1: these previously survived indefinitely in /tmp, silently
# contradicting the UserSov S3 disposition's claim that the raw
# --output-format json object — which contains the free-text `result` field,
# outside the S3 allowlist — is deleted after extraction). Safe to call
# multiple times (idempotent); safe to call when some paths don't exist.
runCampaign.cleanupSlot() {
  local slot=$1
  local slotClone=/tmp/slot-$(printf '%02d' $slot)-clone
  local slotGomodcache=/tmp/slot-$(printf '%02d' $slot)-gomodcache
  chmod -R u+w $slotGomodcache 2>/dev/null ||:
  rm -rf $slotClone $slotGomodcache $slotClone.raw-result.json $slotClone.raw-result.json.stderr
}

# runCampaign.cleanupAllSlots is a BEST-EFFORT crash-safety backstop (grade
# IMPL-F1/R2-6) — registered as an EXIT trap AFTER the campaign lock is
# acquired (grade IMPL-R2-1: never before, so a process that loses the lock
# race never registers this destructive trap against another holder's
# in-flight artifacts). Covers ordinary exits, `set -e` aborts, and most
# signals bash traps — it CANNOT run after SIGKILL, a host crash, or power
# loss, and it does not itself verify an orphaned dispatch process has
# terminated before removing its clone. Not a guarantee; a residual sweep.
runCampaign.cleanupAllSlots() {
  local -i s
  for (( s = 1; s <= 20; s++ )); do
    runCampaign.cleanupSlot $s
  done
}

main() {
  local journalDir=$1 resultsDir=$2
  local oracleWt=$HOME/projects/go-fp-lint-oracle-c9fc0bf
  local refCorrectDir=$Here/refpkgs/correct
  local refBrokenDir=$Here/refpkgs/broken
  local sourceRepo=$HOME/projects/go-fp-lint

  runCampaign.preflight $oracleWt $refCorrectDir $refBrokenDir || { echo 'HALT: pre-flight failed' >&2; return 1; }

  local vectorSha=$(printf '%s' "${FrozenVector[*]}" | sha256sum | cut -d' ' -f1)
  local campaign=$(journal.Init $journalDir $vectorSha)
  echo "campaign: $campaign (vector_sha=$vectorSha)"

  # Grade IMPL-R2-1 fix: acquire the exclusive campaign lock BEFORE
  # registering the cleanup trap. Slot artifact paths are global
  # (/tmp/slot-NN-clone, unnamespaced by campaign), so a process that loses
  # the lock race must NEVER register the destructive cleanup trap — R1's
  # placement (trap registered unconditionally at function entry) meant a
  # losing process still ran cleanupAllSlots on exit and could delete the
  # LOCK-HOLDING process's in-flight clone/gomodcache/raw-result files
  # mid-dispatch. Only the actual lock holder may own cleanup responsibility
  # for these shared paths.
  journal.Lock $journalDir || { echo 'HALT: another campaign process holds the lock on this journal' >&2; return 1; }
  trap runCampaign.cleanupAllSlots EXIT

  local oracleBlobSha=$(cd $oracleWt && git rev-parse HEAD:experiments/causalarm91119/refcorrect.go)

  local -i slot
  for (( slot = 1; slot <= 20; slot++ )); do
    local arm=${FrozenVector[$((slot - 1))]}
    local state=$(journal.SlotState $journalDir $slot)

    if [[ $state == recorded ]]; then
      echo "slot $slot ($arm): already recorded, skipping"
      continue
    fi
    if [[ $state == dispatched ]]; then
      echo "slot $slot ($arm): possibly-launched from a prior crashed run"
      # Grade IMPL-F2/F3 fix: (1) the search path must be the REAL clone path
      # dispatch.Run actually used (/tmp/slot-NN-clone), not the previous
      # broken $journalDir/../... construction, which never matched any real
      # process argv and so could silently "prove" a live delegate dead.
      # (2) proof MUST be confirmed BEFORE the terminal infra-void record is
      # written — recording first meant a failed proof still left the slot
      # permanently closed despite the HALT that was supposed to follow.
      local slotCloneForRecovery=/tmp/slot-$(printf '%02d' $slot)-clone
      local proven=$(journal.KillAndProve $slotCloneForRecovery 30)
      [[ $proven == 1 ]] || { echo "HALT: could not prove slot $slot's process terminated -- NOT recorded, do not resume without manual investigation" >&2; return 1; }
      # Grade IMPL-R3-1 fix: this is the MOST serious of the three
      # previously-unchecked journal.Record sites — a failure here was
      # followed by `continue`, so the loop would proceed to the NEXT slot
      # while this one stayed genuinely unrecorded (state still
      # `dispatched`), risking a misleading eventual "CAMPAIGN COMPLETE"
      # despite an unresolved slot. Now halts instead.
      journal.Record $journalDir $slot infra-void possibly_launched_prior_crash '{}' \
        || { echo "HALT: slot $slot proven dead but journal.Record failed -- state inconsistent, do not resume without manual investigation" >&2; return 1; }
      echo "slot $slot: marked infra-void (possibly-launched, never re-run); continuing at $((slot + 1))"
      continue
    fi

    echo "slot $slot ($arm): dispatching"
    journal.Reserve $journalDir $slot $arm
    journal.Dispatch $journalDir $slot $$

    local slotClone=/tmp/slot-$(printf '%02d' $slot)-clone
    local slotGomodcache=/tmp/slot-$(printf '%02d' $slot)-gomodcache
    runCampaign.cleanupSlot $slot

    if ! dispatch.CloneAndIsolate $sourceRepo preoracle-base $slotClone $oracleBlobSha; then
      journal.Record $journalDir $slot infra-void clone_isolation_failed '{}' \
        || echo "WARNING: slot $slot journal.Record also failed after clone/isolation failure -- journal state may not reflect this halt" >&2
      echo "HALT: slot $slot clone/isolation failed" >&2
      return 1
    fi

    local briefFile=$Here/briefs/arm-$arm.txt
    local rawResult=$slotClone.raw-result.json
    dispatch.Run $slotClone $briefFile $HOME/projects $slotGomodcache $rawResult claude-haiku-4-5-20251001 900

    if [[ ! -f $rawResult ]]; then
      journal.Record $journalDir $slot infra-void dispatch_produced_no_output '{}' \
        || echo "WARNING: slot $slot journal.Record also failed after dispatch produced no output -- journal state may not reflect this halt" >&2
      echo "HALT: slot $slot dispatch produced no raw result" >&2
      return 1
    fi

    local pkgDir_
    if ! pkgDir_=$(runCampaign.discoverDelegatePkg $slotClone); then
      journal.Record $journalDir $slot fail no_matching_module '{}' \
        || { echo "HALT: slot $slot journal.Record failed" >&2; return 1; }
      echo "slot $slot: FAIL (no matching delegate module); continuing"
      runCampaign.cleanupSlot $slot
      continue
    fi

    # Grade IMPL-R2-4 fix: declare-then-assign (not `local x=$(cmd)`, which
    # masks the command substitution's exit code — SC2155) so a
    # processSlot failure (e.g. journal.Record refusing an overwrite) is
    # actually visible and halts, instead of main() silently printing a
    # stale-looking success line.
    local slotOutcome
    slotOutcome=$(runCampaign.processSlot $journalDir $slot $arm "$pkgDir_" $rawResult $oracleWt $refCorrectDir) \
      || { echo "HALT: slot $slot processSlot failed ($slotOutcome)" >&2; runCampaign.cleanupSlot $slot; return 1; }
    echo "slot $slot ($arm): $slotOutcome"
    runCampaign.cleanupSlot $slot

    if [[ ${slotOutcome%% *} == infra-void ]]; then
      echo "HALT: slot $slot infra-void (${slotOutcome#* }) — systemic scorer/infra problem, not a delegate failure" >&2
      return 1
    fi
  done

  echo 'CAMPAIGN COMPLETE: all 20 slots recorded'
}

IFS=$'\n'
set -o noglob
set -uo pipefail

# sourcing-test guard — lets pre-flight/tests source this file and call
# individual functions without running main().
return 2>/dev/null

set -e
main "$@"
