package service

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
)

func (s *Service) createMemoCreateActivity(ctx context.Context, memo *api.Memo) error {
	payload := api.ActivityMemoCreatePayload{
		Content:    memo.Content,
		Visibility: memo.Visibility.String(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: memo.CreatorID,
		Type:      api.ActivityMemoCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createUserCreateActivity(ctx context.Context, user *api.User) error {
	payload := api.ActivityUserCreatePayload{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createUserAuthSignInActivity(ctx context.Context, user *api.User, ip string) error {
	payload := api.ActivityUserAuthSignInPayload{UserID: user.ID, IP: ip}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserAuthSignIn,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createUserAuthSignUpActivity(ctx context.Context, user *api.User, ip string) error {
	payload := api.ActivityUserAuthSignUpPayload{Username: user.Username, IP: ip}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserAuthSignUp,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createResourceCreateActivity(ctx context.Context, resource *api.Resource) error {
	payload := api.ActivityResourceCreatePayload{
		Filename: resource.Filename,
		Type:     resource.Type,
		Size:     resource.Size,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: resource.CreatorID,
		Type:      api.ActivityResourceCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createTagCreateActivity(ctx context.Context, tag *api.Tag) error {
	payload := api.ActivityTagCreatePayload{
		TagName: tag.Name,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: tag.CreatorID,
		Type:      api.ActivityTagCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}

func (s *Service) createShortcutCreateActivity(ctx context.Context, shortcut *api.Shortcut) error {
	payload := api.ActivityShortcutCreatePayload{
		Title:   shortcut.Title,
		Payload: shortcut.Payload,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: shortcut.CreatorID,
		Type:      api.ActivityShortcutCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	// In prod mode, Store.CreateActivity intentionally returns (nil, nil); treat as success.
	_ = activity
	return nil
}
