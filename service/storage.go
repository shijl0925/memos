package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

// CreateStorage checks that the caller is a Host and persists a new storage config.
func (s *Service) CreateStorage(ctx context.Context, userID int, create *api.StorageCreate) (*api.Storage, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	return s.Store.CreateStorage(ctx, create)
}

// UpdateStorage checks that the caller is a Host and applies the patch.
func (s *Service) UpdateStorage(ctx context.Context, userID, storageID int, patch *api.StoragePatch) (*api.Storage, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	patch.ID = storageID
	return s.Store.PatchStorage(ctx, patch)
}

// ListStorages checks that the caller is a Host and returns all storage configs.
func (s *Service) ListStorages(ctx context.Context, userID int) ([]*api.Storage, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	return s.Store.FindStorageList(ctx, &api.StorageFind{})
}

// DeleteStorage checks that the caller is a Host, ensures the storage is not
// currently in use, and deletes it.
func (s *Service) DeleteStorage(ctx context.Context, userID, storageID int) error {
	if err := s.requireHost(ctx, userID); err != nil {
		return err
	}

	systemSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingStorageServiceIDName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return fmt.Errorf("failed to find storage setting: %w", err)
	}
	if systemSetting != nil {
		activeStorageID := api.DatabaseStorage
		if err := json.Unmarshal([]byte(systemSetting.Value), &activeStorageID); err != nil {
			return fmt.Errorf("failed to unmarshal storage service id: %w", err)
		}
		if activeStorageID == storageID {
			return common.Errorf(common.Invalid, fmt.Errorf("storage service %d is in use", storageID))
		}
	}

	return s.Store.DeleteStorage(ctx, &api.StorageDelete{ID: storageID})
}

// requireHost is a convenience helper that returns NotAuthorized when the
// acting user is not the Host.
func (s *Service) requireHost(ctx context.Context, userID int) error {
	user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil || user.Role != api.Host {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return nil
}
