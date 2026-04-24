package service

import (
	"context"
	"fmt"
	"time"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser checks that the acting user has the Host role, validates the
// payload, hashes the password and persists the user.
func (s *Service) CreateUser(ctx context.Context, currentUserID int, create *api.UserCreate) (*api.User, error) {
	currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &currentUserID})
	if err != nil {
		return nil, fmt.Errorf("failed to find current user: %w", err)
	}
	if currentUser.Role != api.Host {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("only host user can create member"))
	}
	if create.Role == api.Host {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("could not create host user"))
	}

	create.OpenID = common.GenUUID()
	if err := create.Validate(); err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(create.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password hash: %w", err)
	}
	create.PasswordHash = string(passwordHash)

	user, err := s.Store.CreateUser(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := s.createUserCreateActivity(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return user, nil
}

// UpdateUser checks that the acting user owns the account (or is a Host), then
// applies the patch.
func (s *Service) UpdateUser(ctx context.Context, currentUserID, userID int, patch *api.UserPatch) (*api.User, error) {
	currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &currentUserID})
	if err != nil {
		return nil, fmt.Errorf("failed to find current user: %w", err)
	}
	if currentUser.Role != api.Host && currentUserID != userID {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("access forbidden for current session user"))
	}

	if patch.Password != nil && *patch.Password != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(*patch.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to generate password hash: %w", err)
		}
		passwordHashStr := string(passwordHash)
		patch.PasswordHash = &passwordHashStr
	}

	if patch.ResetOpenID != nil && *patch.ResetOpenID {
		openID := common.GenUUID()
		patch.OpenID = &openID
	}

	if err := patch.Validate(); err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}

	currentTs := time.Now().Unix()
	patch.ID = userID
	patch.UpdatedTs = &currentTs

	user, err := s.Store.PatchUser(ctx, patch)
	if err != nil {
		return nil, fmt.Errorf("failed to patch user: %w", err)
	}

	userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find user settings: %w", err)
	}
	user.UserSettingList = userSettingList
	return user, nil
}

// DeleteUser checks that the acting user has the Host role and deletes the account.
func (s *Service) DeleteUser(ctx context.Context, currentUserID, userID int) error {
	currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &currentUserID})
	if err != nil {
		return fmt.Errorf("failed to find current user: %w", err)
	}
	if currentUser.Role != api.Host {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("access forbidden for current session user"))
	}
	return s.Store.DeleteUser(ctx, &api.UserDelete{ID: userID})
}

// GetMe returns the current user together with their settings.
func (s *Service) GetMe(ctx context.Context, userID int) (*api.User, error) {
	user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find user settings: %w", err)
	}
	user.UserSettingList = userSettingList
	return user, nil
}

// UpsertUserSetting validates and persists a user setting.
func (s *Service) UpsertUserSetting(ctx context.Context, userID int, upsert *api.UserSettingUpsert) (*api.UserSetting, error) {
	if err := upsert.Validate(); err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}
	upsert.UserID = userID
	return s.Store.UpsertUserSetting(ctx, upsert)
}
