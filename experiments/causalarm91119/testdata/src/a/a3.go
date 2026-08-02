package a

// Case 8 (R4 New-3) — a DIFFERENT fluentfp type with a same-named Map: must NOT
// flag. slice.Mapper is the vehicle target; option.Option is another fluentfp
// type. An analyzer that checks only "receiver package is under
// binaryphile/fluentfp" (a broad-namespace shortcut) would FALSELY flag this,
// passing every other fixture while violating §3's "not another fluentfp type"
// rule. The correct receiver-type-exact reference emits no diagnostic here; the
// name-only reference false-positives (adding to the discrimination signal).
import "github.com/binaryphile/fluentfp/option"

func Case8() { option.Some(1).Map(inc) }
