package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/idp"
	"github.com/usememos/memos/plugin/idp/oauth2"
	"github.com/usememos/memos/store"
	"golang.org/x/crypto/bcrypt"
)

// SignIn verifies credentials and returns the authenticated user.
func (s *Service) SignIn(ctx context.Context, username, password string) (*api.User, error) {
	user, err := s.Store.FindUser(ctx, &api.UserFind{Username: &username})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("incorrect login credentials, please try again")
	}
	if user == nil {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("incorrect login credentials, please try again"))
	}
	if user.RowStatus == api.Archived {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("user has been archived with username %s", username))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("incorrect login credentials, please try again"))
	}
	return user, nil
}

// SignInSSO exchanges an OAuth2 code for user info, auto-creating accounts
// for new users, and returns the authenticated user.
func (s *Service) SignInSSO(ctx context.Context, signin *api.SSOSignIn, ip string) (*api.User, error) {
	identityProviderMessage, err := s.Store.GetIdentityProvider(ctx, &store.FindIdentityProviderMessage{ID: &signin.IdentityProviderID})
	if err != nil {
		return nil, fmt.Errorf("failed to find identity provider: %w", err)
	}

	var userInfo *idp.IdentityProviderUserInfo
	if identityProviderMessage.Type == store.IdentityProviderOAuth2 {
		oauth2IdentityProvider, err := oauth2.NewIdentityProvider(identityProviderMessage.Config.OAuth2Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create identity provider: %w", err)
		}
		token, err := oauth2IdentityProvider.ExchangeToken(ctx, signin.RedirectURI, signin.Code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange token: %w", err)
		}
		userInfo, err = oauth2IdentityProvider.UserInfo(token)
		if err != nil {
			return nil, fmt.Errorf("failed to get user info: %w", err)
		}
	}

	if identifierFilter := identityProviderMessage.IdentifierFilter; identifierFilter != "" {
		identifierFilterRegex, err := regexp.Compile(identifierFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to compile identifier filter: %w", err)
		}
		if !identifierFilterRegex.MatchString(userInfo.Identifier) {
			return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("access denied, identifier does not match the filter"))
		}
	}

	user, err := s.Store.FindUser(ctx, &api.UserFind{Username: &userInfo.Identifier})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("incorrect login credentials, please try again")
	}
	if user == nil {
		credentialText, err := common.RandomString(20)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random password: %w", err)
		}
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(credentialText), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to generate password hash: %w", err)
		}
		userCreate := &api.UserCreate{
			Username:     userInfo.Identifier,
			Role:         api.NormalUser,
			Nickname:     userInfo.DisplayName,
			Email:        userInfo.Email,
			OpenID:       common.GenUUID(),
			PasswordHash: string(hashBytes),
		}
		user, err = s.Store.CreateUser(ctx, userCreate)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}
	if user.RowStatus == api.Archived {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("user has been archived with username %s", userInfo.Identifier))
	}

	if err := s.createUserAuthSignInActivity(ctx, user, ip); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return user, nil
}

// SignUp auto-provisions the first user as Host, then enforces the
// allowSignUp system setting for subsequent registrations.
func (s *Service) SignUp(ctx context.Context, signup *api.SignUp, ip string) (*api.User, error) {
	userCreate := &api.UserCreate{
		Username: signup.Username,
		Role:     api.NormalUser,
		Nickname: signup.Username,
		OpenID:   common.GenUUID(),
		Password: signup.Password,
	}

	hostUserType := api.Host
	existedHostUsers, err := s.Store.FindUserList(ctx, &api.UserFind{Role: &hostUserType})
	if err != nil {
		return nil, fmt.Errorf("failed to find users: %w", err)
	}
	if len(existedHostUsers) == 0 {
		userCreate.Role = api.Host
	} else {
		allowSignUpSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingAllowSignUpName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return nil, fmt.Errorf("failed to find system setting: %w", err)
		}
		allowSignUpSettingValue := false
		if allowSignUpSetting != nil {
			if err := json.Unmarshal([]byte(allowSignUpSetting.Value), &allowSignUpSettingValue); err != nil {
				return nil, fmt.Errorf("failed to unmarshal allowSignUp setting: %w", err)
			}
		}
		if !allowSignUpSettingValue {
			return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("signup is disabled"))
		}
	}

	if err := userCreate.Validate(); err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password hash: %w", err)
	}
	userCreate.PasswordHash = string(hashBytes)

	user, err := s.Store.CreateUser(ctx, userCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := s.createUserAuthSignUpActivity(ctx, user, ip); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return user, nil
}
