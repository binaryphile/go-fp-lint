package a

// Foo is a plain, hand-authored function -- want SkipGenerated to still
// flag it.
func Foo() { // want "found decl"
}
