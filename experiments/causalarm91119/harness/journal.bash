# Naming Policy:
#
# All function and variable names are camelCased.
#
# Public function names begin with uppercase letters, prefixed "journal." so
# they are namespaced. Private function names begin with lowercase letters,
# same prefix.
#
# Local variable names begin with lowercase letters, e.g. localVariable.
# Global variable names begin with uppercase letters and are namespaced with
# the suffix letter J, e.g. GlobalVariableJ.
#
# journal.bash — the crash-safe, exactly-once per-slot journal for the #93569
# causal-arm campaign (prereg §6/§8; grade R1-F1/F3, R2-1/R2-11).
#
# One immutable record file per slot, journal/slot-NN.json, is the SOLE
# source of truth for what happened. State machine per slot:
#   reserved -> dispatched -> captured -> scored -> recorded
# `dispatched` is written BEFORE the delegate process is spawned (grade R2-1),
# so a crash after launch always leaves a `dispatched` record (conservatively
# possibly-launched, never silently lost). All writes are atomic:
# write-to-temp, fsync, rename, fsync the parent directory. The whole
# campaign is additionally protected by an exclusive flock so two driver
# invocations can never race the same journal.

# journal.Init sets up `journalDir`'s campaign file (idempotent) and echoes
# the campaign UUID on stdout. Asserts `vectorSha` against any pre-existing
# campaign.json (refuses to silently swap the frozen vector under an existing
# campaign). Stdout-return, not a nameref out-param (per bash-style-guide
# §"Returning multiple values via stdout") — a single scalar, and namerefs
# risk a circular self-reference if the caller's variable name collides with
# the function's own nameref parameter name.
journal.Init() {
  local journalDir=$1 vectorSha=$2

  mkdir -p $journalDir
  local campaignFile=$journalDir/campaign.json

  if [[ -f $campaignFile ]]; then
    local existingSha=$(jq -r .vector_sha <$campaignFile)
    [[ $existingSha == $vectorSha ]] || {
      echo "journal.Init: FATAL vector SHA mismatch — existing campaign locked to a different vector" >&2
      return 1
    }
    jq -r .campaign <$campaignFile
    return 0
  fi

  local campaign=$(cat /proc/sys/kernel/random/uuid)
  journal.atomicWrite $campaignFile "$(jq -n --arg c $campaign --arg v $vectorSha \
    '{campaign:$c, vector_sha:$v, created_at: (now | todate)}')"
  echo $campaign
}

# journal.Lock acquires the exclusive campaign flock on fd 9, held for the
# life of the calling shell (never explicitly released — process exit frees
# it). Fatals if another process already holds it.
journal.Lock() {
  local journalDir=$1
  exec 9>$journalDir/campaign.lock
  flock -n 9 || { echo 'journal.Lock: campaign already locked by another process' >&2; return 1; }
}

# journal.SlotState echoes the slot's current `state` field, or "none" if no
# record exists yet. (C) — pure read of an immutable-once-recorded file, and
# reads its own generation are stable within one driver invocation.
journal.SlotState() {
  local journalDir=$1 slot=$2
  local f=$journalDir/slot-$(printf '%02d' $slot).json
  [[ -f $f ]] || { echo none; return; }
  jq -r .state <$f
}

# journal.Reserve writes the initial `reserved` record for `slot`/`arm`.
# Refuses if a record already exists (exactly-once at the reserve boundary).
journal.Reserve() {
  local journalDir=$1 slot=$2 arm=$3
  local f=$journalDir/slot-$(printf '%02d' $slot).json
  [[ ! -f $f ]] || { echo "journal.Reserve: slot $slot already has a record ($(jq -r .state <$f))" >&2; return 1; }
  journal.atomicWrite $f "$(jq -n --argjson s $slot --arg a $arm \
    '{slot:$s, arm:$a, state:"reserved"}')"
}

# journal.Dispatch transitions `slot` to `dispatched`, storing the calling
# driver's own pid (`$$`, forensic context only — see dispatch.Run's docstring
# for why this campaign does NOT track a separate delegate process group).
# MUST be called BEFORE dispatch.Run (grade R2-1) — the record reflects
# "about to launch / may have launched", never "launched and we didn't
# record it". Because dispatch.Run is SYNCHRONOUS, a driver crash is the
# ONLY way a slot is left `dispatched` without `recorded` — journal.Record
# always runs immediately after dispatch.Run returns, in the same shell.
journal.Dispatch() {
  local journalDir=$1 slot=$2 driverPid=$3
  local f=$journalDir/slot-$(printf '%02d' $slot).json
  local cur=$(cat $f)
  journal.atomicWrite $f "$(jq --argjson p $driverPid --arg t "$(date -u +%FT%TZ)" \
    '.state="dispatched" | .driver_pid=$p | .dispatched_at=$t' <<<$cur)"
}

# journal.Record writes the TERMINAL, immutable record for `slot` — status,
# reason, and the metrics object (a JSON string). This is the sole source of
# truth `slots.jsonl` and the stream events are derived from.
journal.Record() {
  local journalDir=$1 slot=$2 status=$3 reason=$4 metricsJson=$5
  local f=$journalDir/slot-$(printf '%02d' $slot).json
  local cur=$(cat $f)
  journal.atomicWrite $f "$(jq --arg st $status --arg r $reason --argjson m "$metricsJson" \
    '.state="recorded" | .status=$st | .reason=$r | .metrics=$m | .recorded_at=(now|todate)' <<<$cur)"
}

# journal.PossiblyLaunched reports whether `slot`'s record is `dispatched`
# with no terminal `recorded` state — the ambiguous "may have launched"
# window a crash can leave behind (grade R2-2). Echoes 1/0.
journal.PossiblyLaunched() {
  local journalDir=$1 slot=$2
  local state=$(journal.SlotState $journalDir $slot)
  [[ $state == dispatched ]] && echo 1 || echo 0
}

# journal.KillAndProve handles the ONLY way a slot can be left
# `dispatched`-but-not-`recorded` under dispatch.Run's synchronous design
# (grade R2-2): the driver process itself crashed mid-slot (kill -9, OOM,
# host failure) while its foreground `dojo`/`claude`/`timeout` children were
# orphaned rather than killed. `slotDir` is the slot's clone path, which
# appears in the `dojo --project` invocation's argv — used as a `pgrep -f`
# search key since there is no captured pgid to target directly. Kills any
# match, blocks until none remain (up to `timeoutS`), then reports whether
# termination was proven. Echoes 1 (proven dead / nothing found) or 0 (grade
# R2-2 — caller MUST halt permanently on 0, never proceed to a later slot).
journal.KillAndProve() {
  local slotDir=$1 timeoutS=${2:-30}

  pkill -TERM -f "dojo --project $slotDir " 2>/dev/null ||:
  local -i waited=0
  while (( waited < timeoutS )); do
    pgrep -f "dojo --project $slotDir " >/dev/null 2>&1 || { echo 1; return; }
    sleep 1
    waited+=1
  done
  pkill -KILL -f "dojo --project $slotDir " 2>/dev/null ||:
  sleep 1
  pgrep -f "dojo --project $slotDir " >/dev/null 2>&1 && echo 0 || echo 1
}

# journal.atomicWrite writes `content_` (may be multi-line JSON — jq's
# pretty-printer) to `path` via write-temp, fsync, rename,
# fsync-parent-directory (grade R2-1). Bash writes the temp file directly
# (avoids embedding arbitrary JSON inside a generated script); a small
# python3 helper performs the fsync/rename/dir-fsync, since bash has no
# builtin fsync and python3 is already a hard dependency of this harness
# (stats.py).
journal.atomicWrite() {
  local path=$1 content_=$2
  local tmp=$path.tmp
  printf '%s' "$content_" >$tmp
  python3 -c '
import os, sys
tmp, path = sys.argv[1], sys.argv[2]
with open(tmp, "r+b") as f:
    f.flush()
    os.fsync(f.fileno())
os.replace(tmp, path)
dirfd = os.open(os.path.dirname(os.path.abspath(path)) or ".", os.O_RDONLY)
try:
    os.fsync(dirfd)
finally:
    os.close(dirfd)
' $tmp $path
}
