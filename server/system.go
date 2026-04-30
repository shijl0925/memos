package server

import (
	"encoding/json"
	ninja "github.com/shijl0925/gin-ninja"
	"net/http"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/server/profile"
)

func (s *Server) registerSystemRoutes(r *ninja.Router) {
	ninja.Get(r, "/ping", adaptNinjaJSONHandler(func(c *ninja.Context, _ *struct{}) (*profile.Profile, error) {
		return s.Profile, nil
	}), ninja.SuccessStatus(http.StatusOK))

	ninja.Get(r, "/status", adaptNinjaJSONHandler(func(c *ninja.Context, _ *struct{}) (*api.SystemStatus, error) {
		ctx := c.Request.Context()
		var userID *int
		if id, ok := c.Context.Get(getUserIDContextKey()); ok {
			if id, ok := id.(int); ok {
				userID = &id
			}
		}
		systemStatus, err := s.Service.GetSystemStatus(ctx, userID)
		if err != nil {
			return nil, convertServiceError(err)
		}
		return systemStatus, nil
	}), ninja.SuccessStatus(http.StatusOK))

	ninja.Post(r, "/system/setting", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/system/setting", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/system/vacuum", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		if err := s.Service.VacuumDatabase(ctx, userID); err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}
