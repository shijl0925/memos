package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerTagRoutes(g Group) {
	g.POST("/tag", func(c Context) error {
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
	})

	g.GET("/tag", func(c Context) error {
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
	})

	g.GET("/tag/suggestion", func(c Context) error {
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
	})

	g.POST("/tag/delete", func(c Context) error {
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
	})
}
