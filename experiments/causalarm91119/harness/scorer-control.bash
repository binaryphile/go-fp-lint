# Naming Policy: see journal.bash — same conventions, prefix "scorerControl.".
#
# scorer-control.bash — brackets each slot's scoring with a known-good
# reference (prereg §8 pass/fail contract; grade R1-F2, R2-3). A scorer that
# cannot correctly pass its own reference is not trustworthy to score a
# delegate — running the control BEFORE (and re-checking AFTER) a slot's
# scoring distinguishes "the scorer/oracle infrastructure is broken"
# (infra-void + HALT) from "the delegate's own analyzer is wrong" (FAIL).

# scorerControl.Check runs the frozen known-good reference package
# (`refDir`, normally harness/refpkgs/correct) through the EXACT same
# wire-and-score.sh path used for real slots, against the pinned oracle
# `oracleWt`. Echoes "healthy" or "unhealthy". Uses a throwaway result file.
scorerControl.Check() {
  local refDir=$1 oracleWt=$2 harnessDir=$3
  local tmpResult=$(mktemp -u /tmp/scorer-control-XXXXXX.json)

  bash $harnessDir/wire-and-score.sh $refDir $oracleWt $tmpResult control control ||:

  if [[ -f $tmpResult ]] && [[ $(jq -r .pass <$tmpResult) == true ]]; then
    rm -f $tmpResult
    echo healthy
  else
    rm -f $tmpResult
    echo unhealthy
  fi
}
