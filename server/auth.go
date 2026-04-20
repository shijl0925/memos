package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	metric "github.com/usememos/memos/plugin/metrics"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerAuthRoutes(g Group) {
	g.POST("/auth/signin", func(c Context) error {
		ctx := c.Request().Context()
		signin := &api.SignIn{}
		if err := json.NewDecoder(c.Request().Body).Decode(signin); err != nil {
			return badRequestError("Malformatted signin request", err)
		}

		userFind := &api.UserFind{
			Username: &signin.Username,
		}
		user, err := s.Store.FindUser(ctx, userFind)
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return internalError(fmt.Sprintf("Failed to find user by username %s", signin.Username), err)
		}
		if user == nil {
			return unauthorizedError(fmt.Sprintf("User not found with username %s", signin.Username))
		} else if user.RowStatus == api.Archived {
			return forbiddenError(fmt.Sprintf("User has been archived with username %s", signin.Username))
		}

		// Compare the stored hashed password, with the hashed version of the password that was received.
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(signin.Password)); err != nil {
			// If the two passwords don't match, return a 401 status.
			return newHTTPErrorWithInternal(http.StatusUnauthorized, "Incorrect password", err)
		}

		if err = setUserSession(c, user); err != nil {
			return internalError("Failed to set signin session", err)
		}
		if err := s.createUserAuthSignInActivity(c, user); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode user response", err)
		}
		return nil
	})

	g.POST("/auth/signup", func(c Context) error {
		ctx := c.Request().Context()
		signup := &api.SignUp{}
		if err := json.NewDecoder(c.Request().Body).Decode(signup); err != nil {
			return badRequestError("Malformatted signup request", err)
		}

		hostUserType := api.Host
		hostUserFind := api.UserFind{
			Role: &hostUserType,
		}
		hostUser, err := s.Store.FindUser(ctx, &hostUserFind)
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return internalError("Failed to find host user", err)
		}
		if signup.Role == api.Host && hostUser != nil {
			return unauthorizedError("Site Host existed, please contact the site host to signin account firstly")
		}

		systemSettingAllowSignUpName := api.SystemSettingAllowSignUpName
		allowSignUpSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
			Name: &systemSettingAllowSignUpName,
		})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return internalError("Failed to find system setting", err)
		}

		allowSignUpSettingValue := false
		if allowSignUpSetting != nil {
			err = json.Unmarshal([]byte(allowSignUpSetting.Value), &allowSignUpSettingValue)
			if err != nil {
				return internalError("Failed to unmarshal system setting allow signup", err)
			}
		}
		if !allowSignUpSettingValue && hostUser != nil {
			return unauthorizedError("Site Host existed, please contact the site host to signin account firstly")
		}

		userCreate := &api.UserCreate{
			Username: signup.Username,
			Role:     api.Role(signup.Role),
			Nickname: signup.Username,
			Password: signup.Password,
			OpenID:   common.GenUUID(),
		}
		if err := userCreate.Validate(); err != nil {
			return badRequestError("Invalid user create format", err)
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
		if err != nil {
			return internalError("Failed to generate password hash", err)
		}

		userCreate.PasswordHash = string(passwordHash)

		user, err := s.Store.CreateUser(ctx, userCreate)
		if err != nil {
			return internalError("Failed to create user", err)
		}
		if err := s.createUserAuthSignUpActivity(c, user); err != nil {
			return internalError("Failed to create activity", err)
		}

		err = setUserSession(c, user)
		if err != nil {
			return internalError("Failed to set signup session", err)
		}

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode created user response", err)
		}
		return nil
	})

	g.POST("/auth/signout", func(c Context) error {
		err := removeUserSession(c)
		if err != nil {
			return internalError("Failed to set sign out session", err)
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
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserAuthSignIn,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Collect(ctx, &metric.Metric{
		Name: string(activity.Type),
	})
	return err
}

func (s *Server) createUserAuthSignUpActivity(c Context, user *api.User) error {
	ctx := c.Request().Context()
	payload := api.ActivityUserAuthSignUpPayload{
		Username: user.Username,
		IP:       getClientIP(c),
	}
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserAuthSignUp,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Collect(ctx, &metric.Metric{
		Name: string(activity.Type),
	})
	return err
}
