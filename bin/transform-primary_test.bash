IFS=$'\n'
set -o noglob

sourceScript() {
  source $Script
}

# realGoPath returns the direct nix-store go binary path, bypassing any
# project bin/go nix-wrapper on PATH (confirmed this session to hang under
# ambient PATH resolution -- jeeves #87588 Phase 3a finding). Globbing is
# disabled file-wide (noglob), so this toggles it on locally to expand the
# nix-store wildcard. Filters to entries that actually contain bin/go (not
# .drv files or bootstrap intermediates, which also match the name glob).
realGoPath() (
  set +o noglob
  # shellcheck disable=SC2012 # ls -d against a bin/go suffix is the
  # correct filter here (only entries WITH that subpath match); find's
  # -iname alone matched .drv/bootstrap entries without it (regression
  # caught in this session's own test run).
  ls -d /nix/store/*-go-1.2*/bin/go 2>/dev/null | head -1
)

## computeDisableFlags -- Data/Calculation quadrant

test_computeDisableFlags() {
  local -A caseDefault=([name]='no --only, everything enabled' [only]='' [want]='')
  local -A caseSingle=([name]='single analyzer' [only]='nestedcall' [want]=$(tesht.ListOf -filterloop=false -impuresource=false -impurereach=false -mapshape=false -recvshape=false -aliaswrite=false -chainlambda=false -chainlayout=false -internalmock=false -methodexpr=false -mapfusion=false))
  local -A caseMulti=([name]='two analyzers' [only]='nestedcall,methodexpr' [want]=$(tesht.ListOf -filterloop=false -impuresource=false -impurereach=false -mapshape=false -recvshape=false -aliaswrite=false -chainlambda=false -chainlayout=false -internalmock=false -mapfusion=false))

  subtest() {
    local casename=$1
    # shellcheck disable=SC9003 # eval itself requires a quoted argument regardless of $casename's own safety
    eval "$(tesht.Inherit $casename)"
    sourceScript

    # shellcheck disable=SC9003 # $only CAN be empty (the no-filter case); unquoted expansion would then vanish as an arg entirely, shifting downstream positional params (proven by this session's own regression run)
    computeDisableFlags "$only"
    local got_
    got_=$(printf '%s\n' "${DisableFlags[@]:-}")
    # shellcheck disable=SC9003 # $want is multi-line for the multi-analyzer cases (built via tesht.ListOf); unquoted expansion would word-split it across AssertGot's positional args (proven by this session's own regression run)
    tesht.AssertGot "$got_" "$want"
  }

  tesht.Run ${!case@}
}

## validateOnlyNames -- Controller quadrant (fatal exits the process; tested
## as a subshell invocation, not by asserting on internal state).

test_validateOnlyNames_rejectsUnknownAnalyzer() {
  sourceScript

  local got_
  got_=$(validateOnlyNames nestedcall,bogus-analyzer 2>&1)
  local rc=$?
  tesht.AssertGot $rc 64
  [[ $got_ == *'unknown --only analyzer: bogus-analyzer'* ]] || tesht.Log "expected 'unknown --only analyzer: bogus-analyzer' in: $got_"
}

## passField -- Data/Calculation quadrant

test_passField() {
  sourceScript

  local block=$'compiled=true\ncount=3'

  local got_
  # shellcheck disable=SC9003 # \$block is multi-line by construction; unquoted expansion would word-split it across passField's positional args (proven by this session's own regression run)
  got_=$(passField compiled "$block")
  tesht.AssertGot "$got_" true

  # shellcheck disable=SC9003 # same multi-line hazard as above
  got_=$(passField count "$block")
  tesht.AssertGot "$got_" 3
}

## emitSummary -- Data/Calculation quadrant

test_emitSummary_shortCircuit() {
  sourceScript

  local got_
  got_=$(emitSummary false null null null null)
  tesht.AssertGot "$got_" '{"compiled": false, "post_fix_compiled": null, "pre_count": null, "post_count": null, "residual": null}'
}

test_emitSummary_clean() {
  sourceScript

  local got_
  got_=$(emitSummary true true 0 0 false)
  tesht.AssertGot "$got_" '{"compiled": true, "post_fix_compiled": true, "pre_count": 0, "post_count": 0, "residual": false}'
}

## main -- Controller quadrant, integration tests (Khorikov §6275: happy
## path + key edge case, real go-fp-lint binary, real filesystem).

test_main_nestedcallFixResolvesResidual() {
  local realGo_
  realGo_=$(realGoPath)
  [[ -n $realGo_ ]] || { tesht.Log 'skip: no nix-store go toolchain found'; return; }

  local dir
  tesht.MktempDir dir

  cat > $dir/go.mod <<'END'
module scratchtest

go 1.23
END
  cat > $dir/main.go <<'END'
package main

func f1(x int) int { return x }

func g() int {
	return f1(f1(f1(5)))
}

func main() { _ = g() }
END

  local got_
  got_=$(env -i HOME=$HOME PATH=$(dirname "$realGo_"):/usr/bin:/bin \
    go="$realGo_" GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 \
    GOCACHE=$HOME/.cache/go-build GOMODCACHE=$HOME/go/pkg/mod GOPATH=$HOME/go \
    $(dirname $TESHT_TEST_FILE)/transform-primary --only nestedcall $dir)
  tesht.AssertGot "$got_" '{"compiled": true, "post_fix_compiled": true, "pre_count": 1, "post_count": 0, "residual": false}'
}

test_main_compileFailureShortCircuits() {
  local realGo_
  realGo_=$(realGoPath)
  [[ -n $realGo_ ]] || { tesht.Log 'skip: no nix-store go toolchain found'; return; }

  local dir
  tesht.MktempDir dir

  cat > $dir/go.mod <<'END'
module scratchtest

go 1.23
END
  cat > $dir/main.go <<'END'
package main

func broken() int {
	return undefinedVariable
}
END

  local got_
  got_=$(env -i HOME=$HOME PATH=$(dirname "$realGo_"):/usr/bin:/bin \
    go="$realGo_" GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 \
    GOCACHE=$HOME/.cache/go-build GOMODCACHE=$HOME/go/pkg/mod GOPATH=$HOME/go \
    $(dirname $TESHT_TEST_FILE)/transform-primary --only nestedcall $dir)
  tesht.AssertGot "$got_" '{"compiled": false, "post_fix_compiled": null, "pre_count": null, "post_count": null, "residual": null}'
}

Script=$(dirname $TESHT_TEST_FILE)/transform-primary
