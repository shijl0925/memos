package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	ninja "github.com/shijl0925/gin-ninja"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerTagRoutes(r *ninja.Router) {
	ninja.Post(r, "/tag", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		tagUpsert := &api.TagUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(tagUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post tag request", err)
		}

		tag, err := s.Service.UpsertTag(ctx, userID, tagUpsert)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(tag.Name))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/tag", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusBadRequest, "Missing user id to find tag")
		}

		tagNameList, err := s.Service.ListTagNames(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(tagNameList))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/tag/suggestion", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusBadRequest, "Missing user session")
		}

		tagList, err := s.Service.GetTagSuggestions(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(tagList))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/tag/delete", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		tagDelete := &api.TagDelete{}
		if err := json.NewDecoder(c.Request().Body).Decode(tagDelete); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post tag request", err)
		}

		if err := s.Service.DeleteTag(ctx, userID, tagDelete.Name); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Tag name not found: %s", tagDelete.Name))
			}
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}
