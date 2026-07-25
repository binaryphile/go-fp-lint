// Package emailapi stands in for a genuinely external module dependency in
// tests: a type here, mocked from package "a", is a real system-boundary
// mock (not the same-module smell).
package emailapi

// Gateway represents an external collaborator (e.g. a third-party email
// API) that "a" depends on.
type Gateway interface {
	Send(to, subject, body string) error
}
