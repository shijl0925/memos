package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerShortcutRoutes(g Group) {
	g.POST("/shortcut", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		shortcutCreate := &api.ShortcutCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(shortcutCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post shortcut request", err)
		}

		shortcut, err := s.Service.CreateShortcut(ctx, userID, shortcutCreate)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(shortcut))
	})

	g.PATCH("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		shortcutPatch := &api.ShortcutPatch{}
		if err := json.NewDecoder(c.Request().Body).Decode(shortcutPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch shortcut request", err)
		}

		shortcut, err := s.Service.UpdateShortcut(ctx, userID, shortcutID, shortcutPatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(shortcut))
	})

	g.GET("/shortcut", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusBadRequest, "Missing user id to find shortcut")
		}

		list, err := s.Store.FindShortcutList(ctx, &api.ShortcutFind{CreatorID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch shortcut list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	})

	g.GET("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		shortcut, err := s.Store.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcutID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch shortcut by ID %d", shortcutID), err)
		}
		return c.JSON(http.StatusOK, composeResponse(shortcut))
	})

	g.DELETE("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		if err := s.Service.DeleteShortcut(ctx, userID, shortcutID); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Shortcut ID not found: %d", shortcutID))
			}
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})
}
