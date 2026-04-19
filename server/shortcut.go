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
			return unauthorizedError("Missing user in session")
		}
		shortcutCreate := &api.ShortcutCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(shortcutCreate); err != nil {
			return badRequestError("Malformatted post shortcut request", err)
		}

		shortcutCreate.CreatorID = userID
		shortcut, err := s.Store.CreateShortcut(ctx, shortcutCreate)
		if err != nil {
			return internalError("Failed to create shortcut", err)
		}
		if err := s.createShortcutCreateActivity(c, shortcut); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, shortcut); err != nil {
			return internalError("Failed to encode shortcut response", err)
		}
		return nil
	})

	g.PATCH("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		shortcutFind := &api.ShortcutFind{
			ID: &shortcutID,
		}
		shortcut, err := s.Store.FindShortcut(ctx, shortcutFind)
		if err != nil {
			return internalError("Failed to find shortcut", err)
		}
		if shortcut.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		currentTs := time.Now().Unix()
		shortcutPatch := &api.ShortcutPatch{
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(shortcutPatch); err != nil {
			return badRequestError("Malformatted patch shortcut request", err)
		}

		shortcutPatch.ID = shortcutID
		shortcut, err = s.Store.PatchShortcut(ctx, shortcutPatch)
		if err != nil {
			return internalError("Failed to patch shortcut", err)
		}

		if err := writeJSON(c, shortcut); err != nil {
			return internalError("Failed to encode shortcut response", err)
		}
		return nil
	})

	g.GET("/shortcut", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return badRequestError("Missing user id to find shortcut", nil)
		}

		shortcutFind := &api.ShortcutFind{
			CreatorID: &userID,
		}
		list, err := s.Store.FindShortcutList(ctx, shortcutFind)
		if err != nil {
			return internalError("Failed to fetch shortcut list", err)
		}

		if err := writeJSON(c, list); err != nil {
			return internalError("Failed to encode shortcut list response", err)
		}
		return nil
	})

	g.GET("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		shortcutFind := &api.ShortcutFind{
			ID: &shortcutID,
		}
		shortcut, err := s.Store.FindShortcut(ctx, shortcutFind)
		if err != nil {
			return internalError(fmt.Sprintf("Failed to fetch shortcut by ID %d", *shortcutFind.ID), err)
		}

		if err := writeJSON(c, shortcut); err != nil {
			return internalError("Failed to encode shortcut response", err)
		}
		return nil
	})

	g.DELETE("/shortcut/:shortcutId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
		}

		shortcutFind := &api.ShortcutFind{
			ID: &shortcutID,
		}
		shortcut, err := s.Store.FindShortcut(ctx, shortcutFind)
		if err != nil {
			return internalError("Failed to find shortcut", err)
		}
		if shortcut.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		shortcutDelete := &api.ShortcutDelete{
			ID: &shortcutID,
		}
		if err := s.Store.DeleteShortcut(ctx, shortcutDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Shortcut ID not found: %d", shortcutID), nil)
			}
			return internalError("Failed to delete shortcut", err)
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
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: shortcut.CreatorID,
		Type:      api.ActivityShortcutCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
