package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	metric "github.com/usememos/memos/plugin/metrics"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerUserRoutes(g Group) {
	g.POST("/user", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing auth session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &userID,
		})
		if err != nil {
			return internalError("Failed to find user by id", err)
		}
		if currentUser.Role != api.Host {
			return unauthorizedError("Only Host user can create member")
		}

		userCreate := &api.UserCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(userCreate); err != nil {
			return badRequestError("Malformatted post user request", err)
		}
		if userCreate.Role == api.Host {
			return forbiddenError("Could not create host user")
		}
		userCreate.OpenID = common.GenUUID()

		if err := userCreate.Validate(); err != nil {
			return badRequestError("Invalid user create format", err)
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)
		if err != nil {
			return internalError("Failed to generate password hash", err)
		}

		userCreate.PasswordHash = string(passwordHash)
		user, err := s.Store.CreateUser(ctx, userCreate)
		if err != nil {
			return internalError("Failed to create user", err)
		}
		if err := s.createUserCreateActivity(c, user); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode user response", err)
		}
		return nil
	})

	g.GET("/user", func(c Context) error {
		ctx := c.Request().Context()
		userList, err := s.Store.FindUserList(ctx, &api.UserFind{})
		if err != nil {
			return internalError("Failed to fetch user list", err)
		}

		for _, user := range userList {
			// data desensitize
			user.OpenID = ""
			user.Email = ""
		}

		if err := writeJSON(c, userList); err != nil {
			return internalError("Failed to encode user list response", err)
		}
		return nil
	})

	// GET /api/user/me is used to check if the user is logged in.
	g.GET("/user/me", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing auth session")
		}

		userFind := &api.UserFind{
			ID: &userID,
		}
		user, err := s.Store.FindUser(ctx, userFind)
		if err != nil {
			return internalError("Failed to find user", err)
		}

		userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{
			UserID: userID,
		})
		if err != nil {
			return internalError("Failed to find userSettingList", err)
		}
		user.UserSettingList = userSettingList

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode user response", err)
		}
		return nil
	})

	g.POST("/user/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing auth session")
		}

		userSettingUpsert := &api.UserSettingUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(userSettingUpsert); err != nil {
			return badRequestError("Malformatted post user setting upsert request", err)
		}
		if err := userSettingUpsert.Validate(); err != nil {
			return badRequestError("Invalid user setting format", err)
		}

		userSettingUpsert.UserID = userID
		userSetting, err := s.Store.UpsertUserSetting(ctx, userSettingUpsert)
		if err != nil {
			return internalError("Failed to upsert user setting", err)
		}

		if err := writeJSON(c, userSetting); err != nil {
			return internalError("Failed to encode user setting response", err)
		}
		return nil
	})

	g.GET("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return badRequestError("Malformatted user id", err)
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &id,
		})
		if err != nil {
			return internalError("Failed to fetch user", err)
		}

		if user != nil {
			// data desensitize
			user.OpenID = ""
			user.Email = ""
		}

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode user response", err)
		}
		return nil
	})

	g.PATCH("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("id")), err)
		}
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &currentUserID,
		})
		if err != nil {
			return internalError("Failed to find user", err)
		}
		if currentUser == nil {
			return badRequestError(fmt.Sprintf("Current session user not found with ID: %d", currentUserID), err)
		} else if currentUser.Role != api.Host && currentUserID != userID {
			return forbiddenError("Access forbidden for current session user")
		}

		currentTs := time.Now().Unix()
		userPatch := &api.UserPatch{
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(userPatch); err != nil {
			return badRequestError("Malformatted patch user request", err)
		}
		userPatch.ID = userID
		if err := userPatch.Validate(); err != nil {
			return badRequestError("Invalid user patch format", err)
		}

		if userPatch.Password != nil && *userPatch.Password != "" {
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(*userPatch.Password), bcrypt.DefaultCost)
			if err != nil {
				return internalError("Failed to generate password hash", err)
			}

			passwordHashStr := string(passwordHash)
			userPatch.PasswordHash = &passwordHashStr
		}

		if userPatch.ResetOpenID != nil && *userPatch.ResetOpenID {
			openID := common.GenUUID()
			userPatch.OpenID = &openID
		}

		user, err := s.Store.PatchUser(ctx, userPatch)
		if err != nil {
			return internalError("Failed to patch user", err)
		}

		userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{
			UserID: userID,
		})
		if err != nil {
			return internalError("Failed to find userSettingList", err)
		}
		user.UserSettingList = userSettingList

		if err := writeJSON(c, user); err != nil {
			return internalError("Failed to encode user response", err)
		}
		return nil
	})

	g.DELETE("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &currentUserID,
		})
		if err != nil {
			return internalError("Failed to find user", err)
		}
		if currentUser == nil {
			return badRequestError(fmt.Sprintf("Current session user not found with ID: %d", currentUserID), nil)
		} else if currentUser.Role != api.Host {
			return forbiddenError("Access forbidden for current session user")
		}

		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("id")), err)
		}

		userDelete := &api.UserDelete{
			ID: userID,
		}
		if err := s.Store.DeleteUser(ctx, userDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("User ID not found: %d", userID), nil)
			}
			return internalError("Failed to delete user", err)
		}

		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) createUserCreateActivity(c Context, user *api.User) error {
	ctx := c.Request().Context()
	payload := api.ActivityUserCreatePayload{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserCreate,
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
