package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerStorageRoutes(g Group) {
	g.POST("/storage", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		storageCreate := &api.StorageCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(storageCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post storage request", err)
		}

		storage, err := s.Store.CreateStorage(ctx, storageCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create storage", err)
		}
		return c.JSON(http.StatusOK, composeResponse(storage))
	})

	g.PATCH("/storage/:storageId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		storageID, err := strconv.Atoi(c.Param("storageId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("storageId")), err)
		}

		storagePatch := &api.StoragePatch{ID: storageID}
		if err := json.NewDecoder(c.Request().Body).Decode(storagePatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch storage request", err)
		}

		storage, err := s.Store.PatchStorage(ctx, storagePatch)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to patch storage", err)
		}
		return c.JSON(http.StatusOK, composeResponse(storage))
	})

	g.GET("/storage", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		storageList, err := s.Store.FindStorageList(ctx, &api.StorageFind{})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find storage list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(storageList))
	})

	g.DELETE("/storage/:storageId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		storageID, err := strconv.Atoi(c.Param("storageId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("storageId")), err)
		}

		systemSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingStorageServiceIDName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find storage", err)
		}
		if systemSetting != nil {
			storageServiceID := 0
			if err := json.Unmarshal([]byte(systemSetting.Value), &storageServiceID); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal storage service id", err)
			}
			if storageServiceID == storageID {
				return newHTTPError(http.StatusBadRequest, fmt.Sprintf("Storage service %d is using", storageID))
			}
		}

		if err = s.Store.DeleteStorage(ctx, &api.StorageDelete{ID: storageID}); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Storage ID not found: %d", storageID))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to delete storage", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}
