package server

import (
	"encoding/json"
	"fmt"
	ninja "github.com/shijl0925/gin-ninja"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerStorageRoutes(r *ninja.Router) {
	ninja.Post(r, "/storage", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		storageCreate := &api.StorageCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(storageCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post storage request", err)
		}

		storage, err := s.Service.CreateStorage(ctx, userID, storageCreate)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(storage))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Patch(r, "/storage/:storageId", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		storageID, err := strconv.Atoi(c.Param("storageId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("storageId")), err)
		}

		storagePatch := &api.StoragePatch{}
		if err := json.NewDecoder(c.Request().Body).Decode(storagePatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch storage request", err)
		}

		storage, err := s.Service.UpdateStorage(ctx, userID, storageID, storagePatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(storage))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/storage", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		storageList, err := s.Service.ListStorages(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(storageList))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Delete(r, "/storage/:storageId", adaptNinjaVoidHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		storageID, err := strconv.Atoi(c.Param("storageId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("storageId")), err)
		}

		if err = s.Service.DeleteStorage(ctx, userID, storageID); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Storage ID not found: %d", storageID))
			}
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	}), ninja.ExcludeFromDocs())
}
