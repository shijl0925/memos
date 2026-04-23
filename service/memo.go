package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

// CreateMemo applies default visibility from user settings, enforces the
// disablePublicMemos system setting, validates content length, persists the
// memo and its associated resources, and records an activity.
func (s *Service) CreateMemo(ctx context.Context, userID int, create *api.MemoCreate) (*api.Memo, error) {
	if len(create.Content) > api.MaxContentLength {
		return nil, common.Errorf(common.Invalid, fmt.Errorf("content size overflow, up to 1MB"))
	}

	if create.Visibility == "" {
		userMemoVisibilitySetting, err := s.Store.FindUserSetting(ctx, &api.UserSettingFind{
			UserID: userID,
			Key:    api.UserSettingMemoVisibilityKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find user setting: %w", err)
		}
		if userMemoVisibilitySetting != nil {
			memoVisibility := api.Private
			if err := json.Unmarshal([]byte(userMemoVisibilitySetting.Value), &memoVisibility); err != nil {
				return nil, fmt.Errorf("failed to unmarshal memo visibility setting: %w", err)
			}
			create.Visibility = memoVisibility
		} else {
			create.Visibility = api.Private
		}
	}

	disablePublicMemosSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
		Name: api.SystemSettingDisablePublicMemosName,
	})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("failed to find system setting: %w", err)
	}
	if disablePublicMemosSetting != nil {
		disablePublicMemos := false
		if err := json.Unmarshal([]byte(disablePublicMemosSetting.Value), &disablePublicMemos); err != nil {
			return nil, fmt.Errorf("failed to unmarshal disablePublicMemos setting: %w", err)
		}
		if disablePublicMemos {
			user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
			if err != nil {
				return nil, fmt.Errorf("failed to find user: %w", err)
			}
			if user.Role == api.NormalUser {
				create.Visibility = api.Private
			}
		}
	}

	create.CreatorID = userID
	memo, err := s.Store.CreateMemo(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create memo: %w", err)
	}

	for _, resourceID := range create.ResourceIDList {
		if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
			MemoID:     memo.ID,
			ResourceID: resourceID,
		}); err != nil {
			return nil, fmt.Errorf("failed to upsert memo resource: %w", err)
		}
	}

	if err := s.createMemoCreateActivity(ctx, memo); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}

	return memo, nil
}

// UpdateMemo verifies ownership then applies the patch and resource changes.
func (s *Service) UpdateMemo(ctx context.Context, userID, memoID int, patch *api.MemoPatch) (*api.Memo, error) {
	memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
	if err != nil {
		return nil, fmt.Errorf("failed to find memo: %w", err)
	}
	if memo.CreatorID != userID {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}

	if patch.Content != nil && len(*patch.Content) > api.MaxContentLength {
		return nil, common.Errorf(common.Invalid, fmt.Errorf("content size overflow, up to 1MB"))
	}

	currentTs := time.Now().Unix()
	patch.ID = memoID
	patch.UpdatedTs = &currentTs

	memo, err = s.Store.PatchMemo(ctx, patch)
	if err != nil {
		return nil, fmt.Errorf("failed to patch memo: %w", err)
	}

	for _, resourceID := range patch.ResourceIDList {
		if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
			MemoID:     memo.ID,
			ResourceID: resourceID,
		}); err != nil {
			return nil, fmt.Errorf("failed to upsert memo resource: %w", err)
		}
	}

	return memo, nil
}

// GetMemo fetches a single memo and enforces visibility rules.
// userID is nil when the caller is not authenticated.
func (s *Service) GetMemo(ctx context.Context, userID *int, memoID int) (*api.Memo, error) {
	memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
	if err != nil {
		return nil, err
	}

	if memo.Visibility == api.Private {
		if userID == nil || memo.CreatorID != *userID {
			return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("this memo is private only"))
		}
	} else if memo.Visibility == api.Protected && userID == nil {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("this memo is protected, missing user in session"))
	}

	return memo, nil
}

// ListMemos fetches memos according to the given filter, applying visibility
// restrictions based on whether the caller is authenticated.
func (s *Service) ListMemos(ctx context.Context, currentUserID *int, find *api.MemoFind) ([]*api.Memo, error) {
	if currentUserID == nil {
		if find.CreatorID == nil {
			return nil, common.Errorf(common.Invalid, fmt.Errorf("missing user id to find memo"))
		}
		find.VisibilityList = []api.Visibility{api.Public}
	} else {
		if find.CreatorID == nil {
			find.CreatorID = currentUserID
		} else if *find.CreatorID != *currentUserID {
			find.VisibilityList = []api.Visibility{api.Public, api.Protected}
		}
	}

	return s.Store.FindMemoList(ctx, find)
}

// DeleteMemo verifies ownership and deletes the memo.
func (s *Service) DeleteMemo(ctx context.Context, userID, memoID int) error {
	memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
	if err != nil {
		return err
	}
	if memo.CreatorID != userID {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return s.Store.DeleteMemo(ctx, &api.MemoDelete{ID: memoID})
}

// UpsertMemoOrganizer pins or unpins a memo for the given user.
func (s *Service) UpsertMemoOrganizer(ctx context.Context, userID, memoID int, upsert *api.MemoOrganizerUpsert) (*api.Memo, error) {
	upsert.MemoID = memoID
	upsert.UserID = userID
	if err := s.Store.UpsertMemoOrganizer(ctx, upsert); err != nil {
		return nil, fmt.Errorf("failed to upsert memo organizer: %w", err)
	}
	return s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
}

// BindMemoResource validates ownership of a resource and attaches it to a memo.
func (s *Service) BindMemoResource(ctx context.Context, userID, memoID int, upsert *api.MemoResourceUpsert) (*api.Resource, error) {
	resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &upsert.ResourceID})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resource: %w", err)
	}
	if resource == nil {
		return nil, common.Errorf(common.NotFound, fmt.Errorf("resource not found"))
	}
	if resource.CreatorID != userID {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized to bind this resource"))
	}

	upsert.MemoID = memoID
	currentTs := time.Now().Unix()
	upsert.UpdatedTs = &currentTs
	if _, err := s.Store.UpsertMemoResource(ctx, upsert); err != nil {
		return nil, fmt.Errorf("failed to upsert memo resource: %w", err)
	}
	return resource, nil
}

// UnbindMemoResource verifies ownership and removes the resource from the memo.
func (s *Service) UnbindMemoResource(ctx context.Context, userID, memoID, resourceID int) error {
	memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
	if err != nil {
		return fmt.Errorf("failed to find memo: %w", err)
	}
	if memo.CreatorID != userID {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return s.Store.DeleteMemoResource(ctx, &api.MemoResourceDelete{
		MemoID:     &memoID,
		ResourceID: &resourceID,
	})
}

// GetMemoStats returns a list of createdTs values for the memos visible to the caller.
func (s *Service) GetMemoStats(ctx context.Context, currentUserID *int, creatorID int) ([]int64, error) {
	normalStatus := api.Normal
	find := &api.MemoFind{
		RowStatus: &normalStatus,
		CreatorID: &creatorID,
	}

	if currentUserID == nil {
		find.VisibilityList = []api.Visibility{api.Public}
	} else if creatorID != *currentUserID {
		find.VisibilityList = []api.Visibility{api.Public, api.Protected}
	} else {
		find.VisibilityList = []api.Visibility{api.Public, api.Protected, api.Private}
	}

	list, err := s.Store.FindMemoList(ctx, find)
	if err != nil {
		return nil, err
	}

	createdTsList := make([]int64, 0, len(list))
	for _, memo := range list {
		createdTsList = append(createdTsList, memo.CreatedTs)
	}
	return createdTsList, nil
}

// ListAllMemos lists memos visible to any user (public/protected), used by the explore feed.
func (s *Service) ListAllMemos(ctx context.Context, authenticated bool, find *api.MemoFind) ([]*api.Memo, error) {
	if !authenticated {
		find.VisibilityList = []api.Visibility{api.Public}
	} else {
		find.VisibilityList = []api.Visibility{api.Public, api.Protected}
	}
	normalStatus := api.Normal
	find.RowStatus = &normalStatus
	return s.Store.FindMemoList(ctx, find)
}
