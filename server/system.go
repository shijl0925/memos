package server

import (
	"encoding/json"
	"net/http"

	"github.com/usememos/memos/api"
)

func (s *Server) registerSystemRoutes(g Group) {
	g.GET("/ping", func(c Context) error {
		return c.JSON(http.StatusOK, composeResponse(s.Profile))
	})

	g.GET("/status", func(c Context) error {
		ctx := c.Request().Context()
		var userID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			userID = &id
		}
		systemStatus, err := s.Service.GetSystemStatus(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(systemStatus))
	})

	g.POST("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		systemSettingUpsert := &api.SystemSettingUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(systemSettingUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post system setting request", err)
		}

		systemSetting, err := s.Service.UpsertSystemSetting(ctx, userID, systemSettingUpsert)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(systemSetting))
	})

	g.GET("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		systemSettingList, err := s.Service.GetSystemSettingList(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(systemSettingList))
	})

	g.POST("/system/vacuum", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		if err := s.Service.VacuumDatabase(ctx, userID); err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})
}
