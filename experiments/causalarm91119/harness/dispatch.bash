# Naming Policy: see journal.bash — same conventions, prefix "dispatch.".
#
# dispatch.bash — isolated per-slot delegate dispatch (prereg §6; grade
# R1-F6, R2-2/R2-5). Fresh shallow clone at the pre-oracle base, origin
# removed, run under dojo with the oracle + source repo hidden and a
# per-slot-private, discarded module cache. Journals `dispatched` BEFORE the
# process is spawned.

# dispatch.CloneAndIsolate makes a fresh shallow clone of `sourceRepo` at
# `preoracleTag` into `dest`, removes all git remotes (kills the re-fetch
# path back to the oracle — grade R1-F6), and asserts the oracle is
# unreachable both by path and by blob id. Fatals loudly on any isolation
# failure — never dispatches into an unverified clone.
dispatch.CloneAndIsolate() {
  local sourceRepo=$1 preoracleTag=$2 dest=$3 oracleBlobSha=$4

  git clone --depth 1 --no-local --branch $preoracleTag file://$sourceRepo $dest \
    >/tmp/clone-$(basename $dest).log 2>&1 \
    || { echo "dispatch.CloneAndIsolate: clone failed" >&2; cat /tmp/clone-$(basename $dest).log >&2; return 1; }

  git -C $dest remote remove origin 2>/dev/null ||:
  local remotesLeft=$(git -C $dest remote)
  [[ -z $remotesLeft ]] || { echo "dispatch.CloneAndIsolate: remotes still present: $remotesLeft" >&2; return 1; }

  [[ ! -e $dest/experiments/causalarm91119/refcorrect.go ]] \
    || { echo 'dispatch.CloneAndIsolate: oracle path present in clone' >&2; return 1; }

  git -C $dest cat-file -e $oracleBlobSha 2>/dev/null \
    && { echo 'dispatch.CloneAndIsolate: oracle blob reachable in clone object store' >&2; return 1; }

  return 0
}

# dispatch.Run launches `claude` in `slotDir` under dojo with the brief at
# `briefPath` as the prompt, hides `hideDir` (the oracle-bearing source
# tree), gives it a fresh empty per-slot module cache (`gomodcacheDir`,
# caller-owned, discarded after the slot), and writes the raw JSON result to
# `rawResultPath`. SYNCHRONOUS (blocking) — bounded by `timeoutS` internally
# via `timeout`. The caller journals `dispatched` immediately BEFORE calling
# this function (grade R2-1's "journaled before spawn" is satisfied by
# ordering, not by racing an async job).
#
# Deliberately not backgrounded via `setsid ... &` + a captured pgid: probed
# empirically in this environment and found broken — `wait $pid` returns
# immediately (rc=0) without the backgrounded job having actually run at
# all, even for a trivial `sleep 3` (confirmed via direct probe, no dojo/
# claude involved). A synchronous foreground call, proven correct by a
# manual dojo+claude invocation outside this function, is the reliable
# choice for this sandbox; recovery on a harness crash mid-slot is handled
# by the crash-safe journal alone (a `dispatched`-but-not-`recorded` record
# on the next invocation ⇒ infra-void ⇒ HALT — see journal.PossiblyLaunched),
# not by killing a tracked process group.
dispatch.Run() {
  local slotDir=$1 briefPath=$2 hideDir=$3 gomodcacheDir=$4 rawResultPath=$5
  local model=$6 timeoutS=$7

  local brief_=$(cat $briefPath)
  mkdir -p $gomodcacheDir

  dojo --project $slotDir --persist $slotDir --persist $gomodcacheDir --hide $hideDir -- \
    env GOMODCACHE=$gomodcacheDir timeout $timeoutS \
    claude -p "$brief_" --model $model --output-format json --permission-mode bypassPermissions \
    >$rawResultPath 2>$rawResultPath.stderr
}
