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
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerUserRoutes(g Group) {
	g.POST("/user", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing auth session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user by id", err)
		}
		if currentUser.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Only Host user can create member")
		}

		userCreate := &api.UserCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(userCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post user request", err)
		}
		if userCreate.Role == api.Host {
			return newHTTPError(http.StatusForbidden, "Could not create host user")
		}
		userCreate.OpenID = common.GenUUID()

		if err := userCreate.Validate(); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Invalid user create format", err)
		}

		rawCredential := userCreate.Password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(rawCredential), bcrypt.DefaultCost)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate password hash", err)
		}

		userCreate.PasswordHash = string(passwordHash)
		user, err := s.Store.CreateUser(ctx, userCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create user", err)
		}
		if err := s.createUserCreateActivity(c, user); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.GET("/user", func(c Context) error {
		ctx := c.Request().Context()
		userList, err := s.Store.FindUserList(ctx, &api.UserFind{})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch user list", err)
		}
		for _, user := range userList {
			user.OpenID = ""
			user.Email = ""
		}
		return c.JSON(http.StatusOK, composeResponse(userList))
	})

	g.POST("/user/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing auth session")
		}

		userSettingUpsert := &api.UserSettingUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(userSettingUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post user setting upsert request", err)
		}
		if err := userSettingUpsert.Validate(); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Invalid user setting format", err)
		}

		userSettingUpsert.UserID = userID
		userSetting, err := s.Store.UpsertUserSetting(ctx, userSettingUpsert)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert user setting", err)
		}
		return c.JSON(http.StatusOK, composeResponse(userSetting))
	})

	g.GET("/user/me", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing auth session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}

		userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{UserID: userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find userSettingList", err)
		}
		user.UserSettingList = userSettingList
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.GET("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted user id", err)
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &id})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch user", err)
		}

		if user != nil {
			user.OpenID = ""
			user.Email = ""
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.PATCH("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("id")), err)
		}
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &currentUserID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if currentUser == nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("Current session user not found with ID: %d", currentUserID), err)
		}
		if currentUser.Role != api.Host && currentUserID != userID {
			return newHTTPErrorWithInternal(http.StatusForbidden, "Access forbidden for current session user", err)
		}

		currentTs := time.Now().Unix()
		userPatch := &api.UserPatch{UpdatedTs: &currentTs}
		if err := json.NewDecoder(c.Request().Body).Decode(userPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch user request", err)
		}
		userPatch.ID = userID

		if userPatch.Password != nil && *userPatch.Password != "" {
			rawCredential := *userPatch.Password
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(rawCredential), bcrypt.DefaultCost)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate password hash", err)
			}
			passwordHashStr := string(passwordHash)
			setUserPatchHash(userPatch, passwordHashStr)
		}

		if userPatch.ResetOpenID != nil && *userPatch.ResetOpenID {
			openID := common.GenUUID()
			userPatch.OpenID = &openID
		}

		if err := userPatch.Validate(); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Invalid user patch format", err)
		}

		user, err := s.Store.PatchUser(ctx, userPatch)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to patch user", err)
		}

		userSettingList, err := s.Store.FindUserSettingList(ctx, &api.UserSettingFind{UserID: userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find userSettingList", err)
		}
		user.UserSettingList = userSettingList
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.DELETE("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		currentUser, err := s.Store.FindUser(ctx, &api.UserFind{ID: &currentUserID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if currentUser == nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("Current session user not found with ID: %d", currentUserID), err)
		}
		if currentUser.Role != api.Host {
			return newHTTPErrorWithInternal(http.StatusForbidden, "Access forbidden for current session user", err)
		}

		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("id")), err)
		}

		userDelete := &api.UserDelete{ID: userID}
		if err := s.Store.DeleteUser(ctx, userDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("User ID not found: %d", userID))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to delete user", err)
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
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
