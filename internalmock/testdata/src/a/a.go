package a

import "github.com/example/emailapi"

// --- positives: Mock<X> where X is defined in the SAME module (here: the
//     same package) — intra-system mock, design smell ---

// OrderRepository is an internal interface, same package as its mock.
type OrderRepository interface {
	FindByID(id string) (string, error)
}

// MockOrderRepository mocks OrderRepository, which lives in this same
// package/module — the smell go-development-guide.md §6 warns about.
type MockOrderRepository struct{} // want "MockOrderRepository mocks OrderRepository, which is defined within this module"

func (m MockOrderRepository) FindByID(id string) (string, error) { return "", nil }

// UserStore is another internal interface, same package as its mock.
type UserStore struct{}

// MockUserStore mocks UserStore -- also same-module, also flagged.
type MockUserStore struct{} // want "MockUserStore mocks UserStore, which is defined within this module"

// --- negatives: not flagged ---

// MockGateway mocks emailapi.Gateway, a type from a DIFFERENT module (a
// real system boundary) -- not the smell.
type MockGateway struct{}

func (m MockGateway) Send(to, subject, body string) error { return nil }

var _ emailapi.Gateway = MockGateway{}

// MockUnknown has no correlated "Unknown" type anywhere reachable --
// conservative no-op (favors false-negative over false-positive).
type MockUnknown struct{}

// mockLowercase isn't exported/PascalCase after "Mock" -- name doesn't match.
type mockLowercase struct{}

// NotAMock doesn't start with "Mock" at all.
type NotAMock struct{}
