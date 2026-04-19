package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
)

func (s *Server) registerShortcutRoutes(g *gin.RouterGroup) {
	g.POST("/shortcut", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		shortcutCreate := &api.ShortcutCreate{
			CreatorID: userID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(shortcutCreate); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted post shortcut request", err)
			return
		}

		shortcut, err := s.Store.CreateShortcut(shortcutCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create shortcut", err)
			return
		}
		writeJSON(c, shortcut)
	})

	g.PATCH("/shortcut/:shortcutId", func(c *gin.Context) {
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
			return
		}

		shortcutPatch := &api.ShortcutPatch{
			ID: shortcutID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(shortcutPatch); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted patch shortcut request", err)
			return
		}

		shortcut, err := s.Store.PatchShortcut(shortcutPatch)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to patch shortcut", err)
			return
		}
		writeJSON(c, shortcut)
	})

	g.GET("/shortcut", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		shortcutFind := &api.ShortcutFind{
			CreatorID: &userID,
		}
		list, err := s.Store.FindShortcutList(shortcutFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch shortcut list", err)
			return
		}
		writeJSON(c, list)
	})

	g.GET("/shortcut/:shortcutId", func(c *gin.Context) {
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
			return
		}

		shortcutFind := &api.ShortcutFind{
			ID: &shortcutID,
		}
		shortcut, err := s.Store.FindShortcut(shortcutFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch shortcut by ID %d", *shortcutFind.ID), err)
			return
		}
		writeJSON(c, shortcut)
	})

	g.DELETE("/shortcut/:shortcutId", func(c *gin.Context) {
		shortcutID, err := strconv.Atoi(c.Param("shortcutId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("shortcutId")), err)
			return
		}

		shortcutDelete := &api.ShortcutDelete{
			ID: shortcutID,
		}
		if err := s.Store.DeleteShortcut(shortcutDelete); err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to delete shortcut", err)
			return
		}

		c.JSON(http.StatusOK, true)
	})
}
