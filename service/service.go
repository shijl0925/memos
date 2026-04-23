package service

import (
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/store"
)

// Service is the central business-logic layer.
// It holds a reference to the data-access layer (Store) and the server profile,
// and exposes methods that implement domain rules, permission checks and
// cross-cutting concerns (e.g. activity logging).
type Service struct {
	Store   *store.Store
	Profile *profile.Profile
}

// New creates a new Service instance.
func New(s *store.Store, p *profile.Profile) *Service {
	return &Service{Store: s, Profile: p}
}
