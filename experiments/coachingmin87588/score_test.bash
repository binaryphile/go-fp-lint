IFS=$'\n'
set -o noglob

# realGoPath returns the direct nix-store go binary path, bypassing any
# project bin/go nix-wrapper on PATH (same rationale as
# bin/transform-primary_test.bash's own copy -- confirmed to hang under
# ambient PATH resolution, jeeves #87588 Phase 3a finding).
realGoPath() (
  set +o noglob
  # shellcheck disable=SC2012 # ls -d against a bin/go suffix is the
  # correct filter here (only entries WITH that subpath match); find's
  # -iname alone matched .drv/bootstrap entries without it (regression
  # caught in bin/transform-primary_test.bash's own test run).
  ls -d /nix/store/*-go-1.2*/bin/go 2>/dev/null | head -1
)

## score.sh -- Controller quadrant, integration tests (Khorikov §6275: real
## go-fp-lint binary, real filesystem, real go build/vet/test invocations --
## deliberately not mocked, matching bin/transform-primary's own posture).
## Committed fixtures (testfixtures/candidate-*) replace this session's
## earlier /tmp-only manual validation (IMPL grade #1 finding 5).

test_score_cleanCandidate() {
  runScoreFixture candidate-clean \
    '{"contract_compliant": true, "compiled": true, "post_fix_compiled": true, "functional_pass": true, "pre_count": 0, "post_count": 0, "residual": false, "clean": true}'
}

test_score_noncompliantCandidate() {
  runScoreFixture candidate-noncompliant \
    '{"contract_compliant": false, "compiled": true, "post_fix_compiled": null, "functional_pass": null, "pre_count": null, "post_count": null, "residual": null, "clean": false}'
}

test_score_compliantButFunctionallyWrong() {
  runScoreFixture candidate-badfunc \
    '{"contract_compliant": true, "compiled": true, "post_fix_compiled": true, "functional_pass": false, "pre_count": 0, "post_count": 0, "residual": false, "clean": false}'
}

test_score_nonCompilingCandidate() {
  runScoreFixture candidate-nobuild \
    '{"contract_compliant": false, "compiled": false, "post_fix_compiled": null, "functional_pass": null, "pre_count": null, "post_count": null, "residual": null, "clean": false}'
}

# runScoreFixture runs score.sh against a committed testfixtures/<name>
# candidate directory and asserts its stdout matches `want` exactly.
runScoreFixture() {
  local fixture=$1 want=$2

  local realGo_
  realGo_=$(realGoPath)
  [[ -n $realGo_ ]] || { tesht.Log 'skip: no nix-store go toolchain found'; return; }

  local testDir
  testDir=$(dirname $TESHT_TEST_FILE)

  local got_
  got_=$(env -i HOME=$HOME PATH=$(dirname "$realGo_"):/usr/bin:/bin:$HOME/.nix-profile/bin \
    go="$realGo_" GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 \
    GOCACHE=$HOME/.cache/go-build GOMODCACHE=$HOME/go/pkg/mod GOPATH=$HOME/go \
    $testDir/score.sh $testDir/testfixtures/$fixture)
  tesht.AssertGot "$got_" $want
}
