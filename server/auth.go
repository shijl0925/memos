package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/idp"
	"github.com/usememos/memos/plugin/idp/oauth2"
	"github.com/usememos/memos/store"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerAuthRoutes(g Group) {
	g.POST("/auth/signin", func(c Context) error {
		ctx := c.Request().Context()
		signin := &api.SignIn{}
		if err := json.NewDecoder(c.Request().Body).Decode(signin); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signin request", err)
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{Username: &signin.Username})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPError(http.StatusInternalServerError, "Incorrect login credentials, please try again")
		}
		if user == nil {
			return newHTTPError(http.StatusUnauthorized, "Incorrect login credentials, please try again")
		}
		if user.RowStatus == api.Archived {
			return newHTTPError(http.StatusForbidden, fmt.Sprintf("User has been archived with username %s", signin.Username))
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(signin.Password)); err != nil {
			return newHTTPError(http.StatusUnauthorized, "Incorrect login credentials, please try again")
		}

		if err = setUserSession(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to set signin session", err)
		}
		if err := s.createUserAuthSignInActivity(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signin/sso", func(c Context) error {
		ctx := c.Request().Context()
		signin := &api.SSOSignIn{}
		if err := json.NewDecoder(c.Request().Body).Decode(signin); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signin request", err)
		}

		identityProviderMessage, err := s.Store.GetIdentityProvider(ctx, &store.FindIdentityProviderMessage{
			ID: &signin.IdentityProviderID,
		})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find identity provider", err)
		}

		var userInfo *idp.IdentityProviderUserInfo
		if identityProviderMessage.Type == store.IdentityProviderOAuth2 {
			oauth2IdentityProvider, err := oauth2.NewIdentityProvider(identityProviderMessage.Config.OAuth2Config)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create identity provider instance", err)
			}
			token, err := oauth2IdentityProvider.ExchangeToken(ctx, signin.RedirectURI, signin.Code)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to exchange token", err)
			}
			userInfo, err = oauth2IdentityProvider.UserInfo(token)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to get user info", err)
			}
		}

		identifierFilter := identityProviderMessage.IdentifierFilter
		if identifierFilter != "" {
			identifierFilterRegex, err := regexp.Compile(identifierFilter)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to compile identifier filter", err)
			}
			if !identifierFilterRegex.MatchString(userInfo.Identifier) {
				return newHTTPErrorWithInternal(http.StatusUnauthorized, "Access denied, identifier does not match the filter.", err)
			}
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{Username: &userInfo.Identifier})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPError(http.StatusInternalServerError, "Incorrect login credentials, please try again")
		}
		if user == nil {
			userCreate := &api.UserCreate{
				Username: userInfo.Identifier,
				Role:     api.NormalUser,
				Nickname: userInfo.DisplayName,
				Email:    userInfo.Email,
				OpenID:   common.GenUUID(),
			}
			credentialText, err := common.RandomString(20)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate random password", err)
			}
			hashBytes, err := bcrypt.GenerateFromPassword([]byte(credentialText), bcrypt.DefaultCost)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate password hash", err)
			}
			userCreate.PasswordHash = string(hashBytes)
			user, err = s.Store.CreateUser(ctx, userCreate)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create user", err)
			}
		}
		if user.RowStatus == api.Archived {
			return newHTTPError(http.StatusForbidden, fmt.Sprintf("User has been archived with username %s", userInfo.Identifier))
		}

		if err = setUserSession(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to set signin session", err)
		}
		if err := s.createUserAuthSignInActivity(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signup", func(c Context) error {
		ctx := c.Request().Context()
		signup := &api.SignUp{}
		if err := json.NewDecoder(c.Request().Body).Decode(signup); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signup request", err)
		}

		userCreate := &api.UserCreate{
			Username: signup.Username,
			Role:     api.NormalUser,
			Nickname: signup.Username,
			OpenID:   common.GenUUID(),
		}
		credentialText := signup.Password
		setUserCreateCredential(userCreate, credentialText)
		hostUserType := api.Host
		existedHostUsers, err := s.Store.FindUserList(ctx, &api.UserFind{Role: &hostUserType})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Failed to find users", err)
		}
		if len(existedHostUsers) == 0 {
			userCreate.Role = api.Host
		} else {
			allowSignUpSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
				Name: api.SystemSettingAllowSignUpName,
			})
			if err != nil && common.ErrorCode(err) != common.NotFound {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find system setting", err)
			}

			allowSignUpSettingValue := false
			if allowSignUpSetting != nil {
				if err := json.Unmarshal([]byte(allowSignUpSetting.Value), &allowSignUpSettingValue); err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal system setting allow signup", err)
				}
			}
			if !allowSignUpSettingValue {
				return newHTTPErrorWithInternal(http.StatusUnauthorized, "signup is disabled", err)
			}
		}

		if err := userCreate.Validate(); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Invalid user create format", err)
		}

		hashBytes, err := bcrypt.GenerateFromPassword([]byte(credentialText), bcrypt.DefaultCost)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate password hash", err)
		}

		userCreate.PasswordHash = string(hashBytes)
		user, err := s.Store.CreateUser(ctx, userCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create user", err)
		}
		if err := s.createUserAuthSignUpActivity(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}

		if err := setUserSession(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to set signup session", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signout", func(c Context) error {
		if err := removeUserSession(c); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to set sign out session", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) createUserAuthSignInActivity(c Context, user *api.User) error {
	ctx := c.Request().Context()
	payload := api.ActivityUserAuthSignInPayload{
		UserID: user.ID,
		IP:     getClientIP(c),
	}
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
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}

func (s *Server) createUserAuthSignUpActivity(c Context, user *api.User) error {
	ctx := c.Request().Context()
	payload := api.ActivityUserAuthSignUpPayload{
		Username: user.Username,
		IP:       getClientIP(c),
	}
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
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
