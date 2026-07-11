package b

// ExportedVar is a package-scope var in a package OTHER than the one under
// analysis, used to verify the own-package-only boundary (fixture #13 in
// docs/design.md's impuresource section).
var ExportedVar int
