package service

import (
	"context"
	"fmt"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/store"
)

// CreateIdentityProvider checks Host role and persists a new IDP.
func (s *Service) CreateIdentityProvider(ctx context.Context, userID int, create *api.IdentityProviderCreate) (*api.IdentityProvider, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	msg, err := s.Store.CreateIdentityProvider(ctx, &store.IdentityProviderMessage{
		Name:             create.Name,
		Type:             store.IdentityProviderType(create.Type),
		IdentifierFilter: create.IdentifierFilter,
		Config:           convertIDPConfigToStore(create.Config),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create identity provider: %w", err)
	}
	return convertIDPFromStore(msg), nil
}

// UpdateIdentityProvider checks Host role and applies the patch.
func (s *Service) UpdateIdentityProvider(ctx context.Context, userID, idpID int, patch *api.IdentityProviderPatch) (*api.IdentityProvider, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	msg, err := s.Store.UpdateIdentityProvider(ctx, &store.UpdateIdentityProviderMessage{
		ID:               idpID,
		Type:             store.IdentityProviderType(patch.Type),
		Name:             patch.Name,
		IdentifierFilter: patch.IdentifierFilter,
		Config:           convertIDPConfigToStore(patch.Config),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update identity provider: %w", err)
	}
	return convertIDPFromStore(msg), nil
}

// ListIdentityProviders returns all IDPs; non-Host callers have OAuth2 secrets masked.
func (s *Service) ListIdentityProviders(ctx context.Context, userID *int) ([]*api.IdentityProvider, error) {
	msgList, err := s.Store.ListIdentityProviders(ctx, &store.FindIdentityProviderMessage{})
	if err != nil {
		return nil, fmt.Errorf("failed to list identity providers: %w", err)
	}

	isHost := false
	if userID != nil {
		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: userID})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return nil, fmt.Errorf("failed to find user: %w", err)
		}
		if user != nil && user.Role == api.Host {
			isHost = true
		}
	}

	idpList := make([]*api.IdentityProvider, 0, len(msgList))
	for _, msg := range msgList {
		idp := convertIDPFromStore(msg)
		if !isHost {
			idp.Config.OAuth2Config.ClientSecret = ""
		}
		idpList = append(idpList, idp)
	}
	return idpList, nil
}

// GetIdentityProvider checks Host role and returns a single IDP.
func (s *Service) GetIdentityProvider(ctx context.Context, userID, idpID int) (*api.IdentityProvider, error) {
	if err := s.requireHost(ctx, userID); err != nil {
		return nil, err
	}
	msg, err := s.Store.GetIdentityProvider(ctx, &store.FindIdentityProviderMessage{ID: &idpID})
	if err != nil {
		return nil, fmt.Errorf("failed to get identity provider: %w", err)
	}
	return convertIDPFromStore(msg), nil
}

// DeleteIdentityProvider checks Host role and deletes the IDP.
func (s *Service) DeleteIdentityProvider(ctx context.Context, userID, idpID int) error {
	if err := s.requireHost(ctx, userID); err != nil {
		return err
	}
	return s.Store.DeleteIdentityProvider(ctx, &store.DeleteIdentityProviderMessage{ID: idpID})
}

// --- conversion helpers ---

func convertIDPFromStore(msg *store.IdentityProviderMessage) *api.IdentityProvider {
	return &api.IdentityProvider{
		ID:               msg.ID,
		Name:             msg.Name,
		Type:             api.IdentityProviderType(msg.Type),
		IdentifierFilter: msg.IdentifierFilter,
		Config:           convertIDPConfigFromStore(msg.Config),
	}
}

func convertIDPConfigFromStore(config *store.IdentityProviderConfig) *api.IdentityProviderConfig {
	return &api.IdentityProviderConfig{
		OAuth2Config: &api.IdentityProviderOAuth2Config{
			ClientID:     config.OAuth2Config.ClientID,
			ClientSecret: config.OAuth2Config.ClientSecret,
			AuthURL:      config.OAuth2Config.AuthURL,
			TokenURL:     config.OAuth2Config.TokenURL,
			UserInfoURL:  config.OAuth2Config.UserInfoURL,
			Scopes:       config.OAuth2Config.Scopes,
			FieldMapping: &api.FieldMapping{
				Identifier:  config.OAuth2Config.FieldMapping.Identifier,
				DisplayName: config.OAuth2Config.FieldMapping.DisplayName,
				Email:       config.OAuth2Config.FieldMapping.Email,
			},
		},
	}
}

func convertIDPConfigToStore(config *api.IdentityProviderConfig) *store.IdentityProviderConfig {
	return &store.IdentityProviderConfig{
		OAuth2Config: &store.IdentityProviderOAuth2Config{
			ClientID:     config.OAuth2Config.ClientID,
			ClientSecret: config.OAuth2Config.ClientSecret,
			AuthURL:      config.OAuth2Config.AuthURL,
			TokenURL:     config.OAuth2Config.TokenURL,
			UserInfoURL:  config.OAuth2Config.UserInfoURL,
			Scopes:       config.OAuth2Config.Scopes,
			FieldMapping: &store.FieldMapping{
				Identifier:  config.OAuth2Config.FieldMapping.Identifier,
				DisplayName: config.OAuth2Config.FieldMapping.DisplayName,
				Email:       config.OAuth2Config.FieldMapping.Email,
			},
		},
	}
}
