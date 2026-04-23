package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
)

func (s *Server) registerUserRoutes(g Group) {
	g.POST("/user", func(c Context) error {
		ctx := c.Request().Context()
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing auth session")
		}

		userCreate := &api.UserCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(userCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post user request", err)
		}

		user, err := s.Service.CreateUser(ctx, currentUserID, userCreate)
		if err != nil {
			return convertServiceError(err)
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

		userSetting, err := s.Service.UpsertUserSetting(ctx, userID, userSettingUpsert)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(userSetting))
	})

	g.GET("/user/me", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing auth session")
		}

		user, err := s.Service.GetMe(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
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

		userPatch := &api.UserPatch{}
		if err := json.NewDecoder(c.Request().Body).Decode(userPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch user request", err)
		}

		user, err := s.Service.UpdateUser(ctx, currentUserID, userID, userPatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.DELETE("/user/:id", func(c Context) error {
		ctx := c.Request().Context()
		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		userID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("id")), err)
		}

		if err := s.Service.DeleteUser(ctx, currentUserID, userID); err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})
}
