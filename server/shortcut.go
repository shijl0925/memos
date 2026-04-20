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

		shortcutCreate.CreatorID = userID
		shortcut, err := s.Store.CreateShortcut(ctx, shortcutCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create shortcut", err)
		}
		if err := s.createShortcutCreateActivity(c, shortcut); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
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

		shortcut, err := s.Store.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcutID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find shortcut", err)
		}
		if shortcut.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		currentTs := time.Now().Unix()
		shortcutPatch := &api.ShortcutPatch{
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(shortcutPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch shortcut request", err)
		}

		shortcutPatch.ID = shortcutID
		shortcut, err = s.Store.PatchShortcut(ctx, shortcutPatch)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to patch shortcut", err)
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

		shortcut, err := s.Store.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcutID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find shortcut", err)
		}
		if shortcut.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		shortcutDelete := &api.ShortcutDelete{
			ID: &shortcutID,
		}
		if err := s.Store.DeleteShortcut(ctx, shortcutDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Shortcut ID not found: %d", shortcutID))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to delete shortcut", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) createShortcutCreateActivity(c Context, shortcut *api.Shortcut) error {
	ctx := c.Request().Context()
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
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
