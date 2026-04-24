package service

import (
	"context"
	"fmt"
	"time"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

// CreateShortcut persists a shortcut and records an activity.
func (s *Service) CreateShortcut(ctx context.Context, userID int, create *api.ShortcutCreate) (*api.Shortcut, error) {
	create.CreatorID = userID
	shortcut, err := s.Store.CreateShortcut(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create shortcut: %w", err)
	}
	if err := s.createShortcutCreateActivity(ctx, shortcut); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return shortcut, nil
}

// UpdateShortcut verifies ownership and applies the patch.
func (s *Service) UpdateShortcut(ctx context.Context, userID, shortcutID int, patch *api.ShortcutPatch) (*api.Shortcut, error) {
	shortcut, err := s.Store.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcutID})
	if err != nil {
		return nil, fmt.Errorf("failed to find shortcut: %w", err)
	}
	if shortcut.CreatorID != userID {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}

	currentTs := time.Now().Unix()
	patch.ID = shortcutID
	patch.UpdatedTs = &currentTs

	return s.Store.PatchShortcut(ctx, patch)
}

// DeleteShortcut verifies ownership and deletes the shortcut.
func (s *Service) DeleteShortcut(ctx context.Context, userID, shortcutID int) error {
	shortcut, err := s.Store.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcutID})
	if err != nil {
		return fmt.Errorf("failed to find shortcut: %w", err)
	}
	if shortcut.CreatorID != userID {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return s.Store.DeleteShortcut(ctx, &api.ShortcutDelete{ID: &shortcutID})
}
