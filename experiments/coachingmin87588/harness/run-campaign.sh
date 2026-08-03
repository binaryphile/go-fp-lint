#!/usr/bin/env bash
# run-campaign.sh drives the #87588 coaching-minimum causal-arm campaign
# (jeeves #96780; forked from #93569's causalarm91119/harness/run-campaign.sh,
# cross-vendor grade R1(REJECT)->R2(SEND BACK)->R3(APPROVE)): a pre-flight
# validation suite (integrity replay, stats, mock-delegate dry-run) followed
# by the 20-slot frozen randomization-vector sequential dispatch, gated by a
# campaign-level CAMPAIGN_HALT state machine (see §3 below) that pauses
# dispatch after slot 1 for a UserSov Stage-2 review and halts terminally on
# an infra-void-threshold or scorer-control-failure signal. See
# investigations/2026-08-02-transform-primary-coaching-minimum-preregistration.md
# and ~/.claude/plans/96780-cheerful-nibbling-sprout.md for the frozen
# protocol this mechanically executes.

# shellcheck disable=SC2128 # BASH_SOURCE always has at least one element
Here=$(cd $(dirname $BASH_SOURCE) && pwd)
source $Here/journal.bash
source $Here/scorer-control.bash
source $Here/dispatch.bash

# The frozen randomization vector (jeeves #96780 design §1 R2 fix — resolved
# NOW, at Implementation Gate time, not "whatever HEAD/seed is" at run time):
# seed 87588, slot i -> arm. C = coaching, U = uncoached.
FrozenVector=( C C C U C C U C U C U U U U U C U C U C )

# runCampaign.classify determines a slot's terminal status from the scorer-
# control health (before/after) and the delegate's own score result (grade
# R1-F2, R2-3, corrected, forked unchanged from #93569): a scorer-control
# failure at EITHER bracket means the scorer/oracle infrastructure itself is
# suspect for this slot -> HALT. Otherwise, a delegate that produced no
# result file, or one that produced a result file with pass=false, is a
# genuine delivery FAIL (never infra-void) — the classifier does NOT
# distinguish "delegate compile error" from "delegate panic" from "wrong
# analyzer": all are FAIL once the scorer is proven healthy on both sides.
# `resultFileExists` is a THREE-state value — "yes" / "no" / "malformed"
# (grade IMPL-R2-2): "malformed" means the scorer process exited and left a
# file, but it failed the shape check — scorer/harness corruption, NOT a
# delegate failure. Echoes "status reason infra_void" (three words) —
# `infra_void` (jeeves #96780 design §1/§3) is the wrapper's own
# compiled&&!post_fix_compiled bridge field, read here alongside the
# existing pass/fail check but NOT itself a pass/fail determinant (a
# pipeline-broke-compilation slot already classifies as
# `fail scored_not_pass` via the existing resultPass read) — its only
# consumer is the campaign-level halt-threshold count in §3, which reads it
# from the durable journal record `runCampaign.processSlot` writes, not from
# this in-memory echo.
runCampaign.classify() {
  local controlBeforeHealthy=$1 controlAfterHealthy=$2 resultFileExists=$3 resultPass=$4
  local resultInfraVoid=${5:-false}

  if [[ $controlBeforeHealthy != healthy ]]; then
    echo "infra-void scorer_control_before_failed $resultInfraVoid"
    return
  fi
  if [[ $controlAfterHealthy != healthy ]]; then
    echo "infra-void scorer_control_after_failed $resultInfraVoid"
    return
  fi
  if [[ $resultFileExists == malformed ]]; then
    echo "infra-void scorer_result_malformed $resultInfraVoid"
    return
  fi
  if [[ $resultFileExists != yes ]]; then
    echo "fail no_result_file_scorer_healthy $resultInfraVoid"
    return
  fi
  if [[ $resultPass == true ]]; then
    echo "pass scored $resultInfraVoid"
  else
    echo "fail scored_not_pass $resultInfraVoid"
  fi
}

# runCampaign.discoverDelegatePkg searches `root` for exactly one go.mod
# declaring `module example.com/delegate/coachingmin87588` (prereg §3/§8's
# frozen delivery contract; grade R1-F5 — deterministic, no location hints).
# Echoes the containing directory on success; echoes nothing and returns 1
# on zero-or-multiple matches (the caller classifies this as delivery FAIL).
runCampaign.discoverDelegatePkg() {
  local root=$1
  local matches=$(grep -rl '^module example\.com/delegate/coachingmin87588$' $root --include=go.mod 2>/dev/null ||:)
  local -i n=$(wc -l <<<"${matches:-}")
  [[ -z $matches ]] && n=0
  if (( n != 1 )); then
    echo "runCampaign.discoverDelegatePkg: found $n candidate module(s), need exactly 1" >&2
    return 1
  fi
  dirname $matches
}

## §3 — campaign-level halt-state machine (jeeves #96780 design §3; R1 P0,
## R2 P0 fixes). A single `journal/CAMPAIGN_HALT` file, `{reason, at}`,
## first-writer-wins — never overwritten once present. Checked before every
## slot, including slot 1. Three writers, two terminal:
##   awaiting-usersov-stage2   — after slot 1 is recorded; the ONLY
##                               clearable reason (runCampaign.clearHalt).
##   infra-void-threshold-<arm> — 2nd infra_void:true in either arm.
##   scorer-control-failure     — a classify infra-void status (scorer/oracle
##                               infra itself suspect).

# runCampaign.checkHalt echoes the current CAMPAIGN_HALT reason, or the
# empty string if no marker exists yet. (C)
runCampaign.checkHalt() {
  local journalDir=$1
  local f=$journalDir/CAMPAIGN_HALT
  [[ -f $f ]] || { echo ''; return; }
  jq -r .reason <$f
}

# runCampaign.writeHalt writes `journalDir/CAMPAIGN_HALT` with `reason`,
# first-writer-wins — a no-op if a marker already exists, so the FIRST
# triggering reason is always the one that sticks (grade R2 unified-halt
# design: two of the three reasons are terminal, so a second cause can never
# actually arise once the first has stopped the loop).
runCampaign.writeHalt() {
  local journalDir=$1 reason=$2
  local f=$journalDir/CAMPAIGN_HALT
  [[ ! -f $f ]] || return 0
  journal.atomicWrite $f "$(jq -n --arg r $reason '{reason:$r, at:(now|todate)}')"
}

# runCampaign.clearHalt removes `journalDir/CAMPAIGN_HALT`, but ONLY when
# its current reason is "awaiting-usersov-stage2" — the sole clearable
# reason. Fatals rather than silently no-op on any other state, so a caller
# can never accidentally clear a terminal halt.
runCampaign.clearHalt() {
  local journalDir=$1
  local f=$journalDir/CAMPAIGN_HALT
  [[ -f $f ]] || { echo 'runCampaign.clearHalt: no CAMPAIGN_HALT marker present' >&2; return 1; }
  local reason=$(jq -r .reason <$f)
  [[ $reason == awaiting-usersov-stage2 ]] \
    || { echo "runCampaign.clearHalt: refusing to clear terminal reason '$reason'" >&2; return 1; }
  rm -f $f
}

# runCampaign.infraVoidCount echoes the count of RECORDED slots in `arm`
# whose scored result had infra_void:true, scanned from the durable journal
# (not in-memory state) so the threshold survives a crash/resume across
# multiple `run-campaign.sh` invocations. (C)
runCampaign.infraVoidCount() {
  local journalDir=$1 arm=$2
  local slotFiles
  mapfile -t slotFiles < <(find $journalDir -maxdepth 1 -name 'slot-*.json' 2>/dev/null | sort)
  local -i n=0
  local f
  for f in "${slotFiles[@]}"; do
    [[ $(jq -r '.arm // empty' <$f) == $arm ]] || continue
    [[ $(jq -r '.state // empty' <$f) == recorded ]] || continue
    [[ $(jq -r '.metrics.infra_void // false' <$f) == true ]] && n+=1
  done
  echo $n
}

# runCampaign.postRecordChecks evaluates Trigger 1 (slot 1 recorded) and
# Trigger 2 (infra-void threshold) after `slot`/`arm` reaches a `recorded`
# state, writing CAMPAIGN_HALT if either fires. Does NOT itself halt the
# loop — the NEXT iteration's top-of-loop `runCampaign.checkHalt` call is
# what actually stops dispatch, so this function's only job is detection.
runCampaign.postRecordChecks() {
  local journalDir=$1 slot=$2 arm=$3

  if (( slot == 1 )); then
    runCampaign.writeHalt $journalDir awaiting-usersov-stage2
  fi

  local -i voidCount=$(runCampaign.infraVoidCount $journalDir $arm)
  if (( voidCount >= 2 )); then
    runCampaign.writeHalt $journalDir infra-void-threshold-$arm
  fi
}

# runCampaign.processSlot runs ONE slot (`slot`/`arm`) through the shared
# discovery -> scorer-control -> score -> classify -> journal pipeline.
# `delegatePkgDir` and `rawResultJson` are ALREADY PRODUCED by the caller —
# either a live dispatch.Run (real slot) or a canned mock (dry-run
# validation) — so this function's body is IDENTICAL for both (grade
# R2-9's "mock enters downstream and shares the live path's code"). Forked
# from #93569 unchanged except: wire-and-score.sh is this harness's own
# thin score.sh wrapper (jeeves #96780 design §1), and the scored result's
# `infra_void` bridge field is now read and threaded into the journal
# record alongside pass/mismatches (design §1/§3).
runCampaign.processSlot() {
  local journalDir=$1 slot=$2 arm=$3 delegatePkgDir=$4 rawResultJson=$5
  local oracleWt=$6 refCorrectDir=$7

  local claudeMetrics_=$(python3 $Here/parse_claude_result.py $rawResultJson)

  local controlBefore=$(scorerControl.Check $refCorrectDir $oracleWt $Here)

  local scoreResult=/tmp/score-slot-$(printf '%02d' $slot)-$$.json
  rm -f $scoreResult
  local resultExists=no resultPass=false mismatches='[]' resultInfraVoid=false
  bash $Here/wire-and-score.sh $delegatePkgDir $oracleWt $scoreResult $journalDir slot$slot 2>/dev/null ||:
  # Grade IMPL-F8/R2-2/R3-2 fix (forked unchanged): the result file is
  # atomically written but was previously consumed via bare `jq -r .pass`
  # with no shape/type check. Validate the required fields exist with the
  # expected types before trusting the file. A file that EXISTS but fails
  # this check is scorer/harness corruption, not a delegate failure.
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
    resultInfraVoid=$(jq -r '.infra_void // false' <$scoreResult)
  fi

  local controlAfter=$(scorerControl.Check $refCorrectDir $oracleWt $Here)

  local classified=$(runCampaign.classify $controlBefore $controlAfter $resultExists $resultPass $resultInfraVoid)
  local status=${classified%% *}
  local classifiedRest=${classified#* }
  local reason=${classifiedRest%% *}

  local combinedMetrics=$(jq -n --argjson c "$claudeMetrics_" --argjson p $resultPass \
    --argjson m "$mismatches" --arg cb $controlBefore --arg ca $controlAfter --argjson iv $resultInfraVoid \
    '$c + {score_pass:$p, mismatches:$m, control_before:$cb, control_after:$ca, infra_void:$iv}')

  # Grade IMPL-R2-4 fix (forked unchanged): propagate journal.Record's
  # failure explicitly rather than letting `local x=$(...)` mask it.
  journal.Record $journalDir $slot $status $reason "$combinedMetrics" || {
    rm -f $scoreResult
    echo "infra-void journal_record_failed $resultInfraVoid"
    return 1
  }
  rm -f $scoreResult

  echo "$status $reason $resultInfraVoid"
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
  # Anchor: campaign-1 (2026-08-02, #93569) discovered empirically that
  # `dojo --project X` does NOT change the wrapped command's cwd -- 5/5
  # delegates produced real, substantial work that landed outside the
  # persisted directory and was silently discarded on sandbox exit. This
  # check would have caught it before any real spend.
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
  local actualHash=$( (cd $oracleWt && find experiments/coachingmin87588 -type f | sort | xargs sha256sum | sha256sum | cut -d' ' -f1) )
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
  local uSha=$(sha256sum $Here/briefs/arm-U.txt | cut -d' ' -f1)
  local cSha=$(sha256sum $Here/briefs/arm-C.txt | cut -d' ' -f1)
  local frozenU=$(jq -r .briefs.arm_U_sha256 <$Here/FROZEN.json)
  local frozenC=$(jq -r .briefs.arm_C_sha256 <$Here/FROZEN.json)
  [[ $uSha == $frozenU ]] || { echo "FAIL: arm-U brief SHA drift"; return 1; }
  [[ $cSha == $frozenC ]] || { echo "FAIL: arm-C brief SHA drift"; return 1; }

  echo 'PRE-FLIGHT 7/7: mock-delegate dry-run (shares live processSlot code)'
  local mockJournal=/tmp/preflight-mock-journal-$$
  rm -rf $mockJournal
  journal.Init $mockJournal preflight-mock-vector >/dev/null
  journal.Reserve $mockJournal 1 C
  journal.Dispatch $mockJournal 1 $$
  local mockResult=$(runCampaign.processSlot $mockJournal 1 C $refCorrectDir $Here/fixtures/claude-result-schema.json $oracleWt $refCorrectDir)
  [[ $mockResult == 'pass scored false' ]] || { echo "FAIL: mock-delegate dry-run expected 'pass scored false', got '$mockResult'"; return 1; }
  rm -rf $mockJournal
  echo 'mock-delegate dry-run OK'

  echo 'PRE-FLIGHT: ALL CHECKS PASSED — no vector slots consumed'
  return 0
}

# runCampaign.cleanupSlot removes ALL per-slot ephemeral artifacts for `slot`
# — the clone, the per-slot module cache, AND the raw claude result files
# (grade IMPL-F1, forked unchanged). Safe to call multiple times (idempotent);
# safe to call when some paths don't exist.
runCampaign.cleanupSlot() {
  local slot=$1
  local slotClone=/tmp/slot-$(printf '%02d' $slot)-clone
  local slotGomodcache=/tmp/slot-$(printf '%02d' $slot)-gomodcache
  chmod -R u+w $slotGomodcache 2>/dev/null ||:
  rm -rf $slotClone $slotGomodcache $slotClone.raw-result.json $slotClone.raw-result.json.stderr
}

# runCampaign.cleanupAllSlots is a BEST-EFFORT crash-safety backstop (grade
# IMPL-F1/R2-6, forked unchanged) — registered as an EXIT trap AFTER the
# campaign lock is acquired (grade IMPL-R2-1: never before, so a process
# that loses the lock race never registers this destructive trap against
# another holder's in-flight artifacts). Not a guarantee; a residual sweep.
runCampaign.cleanupAllSlots() {
  local -i s
  for (( s = 1; s <= 20; s++ )); do
    runCampaign.cleanupSlot $s
  done
}

main() {
  local journalDir=$1 resultsDir=$2
  local oracleWt=$HOME/projects/go-fp-lint-oracle-16991c9
  local refCorrectDir=$Here/../testfixtures/candidate-clean
  local refBrokenDir=$Here/../testfixtures/candidate-badfunc
  local sourceRepo=$HOME/projects/go-fp-lint

  runCampaign.preflight $oracleWt $refCorrectDir $refBrokenDir || { echo 'HALT: pre-flight failed' >&2; return 1; }

  local vectorSha=$(printf '%s' "${FrozenVector[*]}" | sha256sum | cut -d' ' -f1)
  # Grade IMPL-R4-1 fix (forked unchanged): declare-then-assign so a
  # journal.Init failure halts instead of silently proceeding.
  local campaign
  campaign=$(journal.Init $journalDir $vectorSha) \
    || { echo 'HALT: journal.Init failed' >&2; return 1; }
  echo "campaign: $campaign (vector_sha=$vectorSha)"

  # Grade IMPL-R2-1 fix (forked unchanged): acquire the exclusive campaign
  # lock BEFORE registering the cleanup trap — slot artifact paths are
  # global, unnamespaced by campaign, so a process that loses the lock race
  # must NEVER register the destructive cleanup trap.
  journal.Lock $journalDir || { echo 'HALT: another campaign process holds the lock on this journal' >&2; return 1; }
  trap runCampaign.cleanupAllSlots EXIT

  # Grade IMPL-R4-1 fix (forked unchanged): declare-then-assign — an empty
  # oracleBlobSha would let dispatch.CloneAndIsolate's isolation check
  # compare against an empty string instead of halting.
  local oracleBlobSha
  oracleBlobSha=$(cd $oracleWt && git rev-parse HEAD:experiments/coachingmin87588/oracle.go) \
    || { echo 'HALT: could not resolve the oracle blob SHA' >&2; return 1; }

  local -i slot
  for (( slot = 1; slot <= 20; slot++ )); do
    # §3 Trigger check — before every slot, including slot 1. A prior
    # iteration's runCampaign.postRecordChecks may have written a marker;
    # this is what actually stops the loop (postRecordChecks only detects).
    local existingHalt=$(runCampaign.checkHalt $journalDir)
    if [[ -n $existingHalt ]]; then
      if [[ $existingHalt == awaiting-usersov-stage2 ]]; then
        echo "PAUSED: campaign halted for UserSov Stage-2 review of slot 1 (reason: $existingHalt) — clear via the Stage-2 clearance interaction (runCampaign.clearHalt), then re-run to continue at slot $slot"
        return 0
      fi
      echo "HALT: campaign halted (reason: $existingHalt) — terminal, not clearable" >&2
      return 1
    fi

    local arm=${FrozenVector[$((slot - 1))]}
    # Grade IMPL-R4-1 fix (forked unchanged): declare-then-assign.
    local state
    state=$(journal.SlotState $journalDir $slot) \
      || { echo "HALT: could not determine slot $slot's state" >&2; return 1; }

    if [[ $state == recorded ]]; then
      echo "slot $slot ($arm): already recorded, skipping"
      continue
    fi
    if [[ $state == dispatched ]]; then
      echo "slot $slot ($arm): possibly-launched from a prior crashed run"
      # Grade IMPL-F2/F3 fix (forked unchanged): the search path is the
      # REAL clone path dispatch.Run used; proof is confirmed BEFORE the
      # terminal infra-void record is written.
      local slotCloneForRecovery=/tmp/slot-$(printf '%02d' $slot)-clone
      local proven=$(journal.KillAndProve $slotCloneForRecovery 30)
      [[ $proven == 1 ]] || { echo "HALT: could not prove slot $slot's process terminated -- NOT recorded, do not resume without manual investigation" >&2; return 1; }
      journal.Record $journalDir $slot infra-void possibly_launched_prior_crash '{}' \
        || { echo "HALT: slot $slot proven dead but journal.Record failed -- state inconsistent, do not resume without manual investigation" >&2; return 1; }
      echo "slot $slot: marked infra-void (possibly-launched, never re-run); continuing at $((slot + 1))"
      runCampaign.postRecordChecks $journalDir $slot $arm
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
      runCampaign.postRecordChecks $journalDir $slot $arm
      continue
    fi

    # Grade IMPL-R2-4 fix (forked unchanged): declare-then-assign so a
    # processSlot failure is actually visible and halts.
    local slotOutcome
    slotOutcome=$(runCampaign.processSlot $journalDir $slot $arm "$pkgDir_" $rawResult $oracleWt $refCorrectDir) \
      || { echo "HALT: slot $slot processSlot failed ($slotOutcome)" >&2; runCampaign.cleanupSlot $slot; return 1; }
    echo "slot $slot ($arm): $slotOutcome"
    runCampaign.cleanupSlot $slot

    if [[ ${slotOutcome%% *} == infra-void ]]; then
      # Trigger 3 (jeeves #96780 design §3) — write the unified marker
      # BEFORE returning, so a resumed invocation refuses at the top-of-loop
      # check rather than silently re-attempting the next slot despite
      # suspect scorer/oracle infra.
      runCampaign.writeHalt $journalDir scorer-control-failure
      echo "HALT: slot $slot infra-void (${slotOutcome#* }) — systemic scorer/infra problem, not a delegate failure" >&2
      return 1
    fi

    runCampaign.postRecordChecks $journalDir $slot $arm
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
