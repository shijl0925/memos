package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/usememos/memos/api"
)

func (s *Server) registerMemoRoutes(g Group) {
	g.POST("/memo", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		memoCreate := &api.MemoCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post memo request", err)
		}

		memo, err := s.Service.CreateMemo(ctx, userID, memoCreate)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(memo))
	})

	g.PATCH("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		memoPatch := &api.MemoPatch{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch memo request", err)
		}

		memo, err := s.Service.UpdateMemo(ctx, userID, memoID, memoPatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(memo))
	})

	g.GET("/memo", func(c Context) error {
		ctx := c.Request().Context()
		memoFind := &api.MemoFind{}
		if userID, err := strconv.Atoi(c.QueryParam("creatorId")); err == nil {
			memoFind.CreatorID = &userID
		}

		var currentUserID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			currentUserID = &id
		}

		rowStatus := api.RowStatus(c.QueryParam("rowStatus"))
		if rowStatus != "" {
			memoFind.RowStatus = &rowStatus
		}
		if pinnedStr := c.QueryParam("pinned"); pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		if tag := c.QueryParam("tag"); tag != "" {
			contentSearch := "#" + tag
			memoFind.ContentSearch = &contentSearch
		}
		if visibilityListStr := c.QueryParam("visibility"); visibilityListStr != "" {
			visibilityList := []api.Visibility{}
			for _, visibility := range strings.Split(visibilityListStr, ",") {
				visibilityList = append(visibilityList, api.Visibility(visibility))
			}
			memoFind.VisibilityList = visibilityList
		}
		if limit, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
			memoFind.Limit = &limit
		}
		if offset, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
			memoFind.Offset = &offset
		}

		list, err := s.Service.ListMemos(ctx, currentUserID, memoFind)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	})

	g.GET("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		var currentUserID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			currentUserID = &id
		}

		memo, err := s.Service.GetMemo(ctx, currentUserID, memoID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(memo))
	})

	g.POST("/memo/:memoId/organizer", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		memoOrganizerUpsert := &api.MemoOrganizerUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoOrganizerUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post memo organizer request", err)
		}

		memo, err := s.Service.UpsertMemoOrganizer(ctx, userID, memoID, memoOrganizerUpsert)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(memo))
	})

	g.POST("/memo/:memoId/resource", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		memoResourceUpsert := &api.MemoResourceUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoResourceUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post memo resource request", err)
		}

		resource, err := s.Service.BindMemoResource(ctx, userID, memoID, memoResourceUpsert)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	})

	g.GET("/memo/:memoId/resource", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		resourceList, err := s.Store.FindResourceList(ctx, &api.ResourceFind{MemoID: &memoID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(resourceList))
	})

	g.GET("/memo/stats", func(c Context) error {
		ctx := c.Request().Context()
		creatorID, err := strconv.Atoi(c.QueryParam("creatorId"))
		if err != nil {
			return newHTTPError(http.StatusBadRequest, "Missing user id to find memo")
		}

		var currentUserID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			currentUserID = &id
		}

		createdTsList, err := s.Service.GetMemoStats(ctx, currentUserID, creatorID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(createdTsList))
	})

	g.GET("/memo/all", func(c Context) error {
		ctx := c.Request().Context()
		memoFind := &api.MemoFind{}

		_, authenticated := c.Get(getUserIDContextKey()).(int)

		if pinnedStr := c.QueryParam("pinned"); pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		if tag := c.QueryParam("tag"); tag != "" {
			contentSearch := "#" + tag
			memoFind.ContentSearch = &contentSearch
		}
		// Free-text search (takes precedence over tag when both are supplied)
		if text := c.QueryParam("text"); text != "" {
			memoFind.ContentSearch = &text
		}
		if visibilityListStr := c.QueryParam("visibility"); visibilityListStr != "" {
			visibilityList := []api.Visibility{}
			for _, visibility := range strings.Split(visibilityListStr, ",") {
				visibilityList = append(visibilityList, api.Visibility(visibility))
			}
			memoFind.VisibilityList = visibilityList
		}
		if from, err := strconv.ParseInt(c.QueryParam("from"), 10, 64); err == nil {
			memoFind.CreatedTsAfter = &from
		}
		if to, err := strconv.ParseInt(c.QueryParam("to"), 10, 64); err == nil {
			memoFind.CreatedTsBefore = &to
		}
		if limit, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
			memoFind.Limit = &limit
		}
		if offset, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
			memoFind.Offset = &offset
		}

		list, err := s.Service.ListAllMemos(ctx, authenticated, memoFind)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	})

	g.DELETE("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		if err := s.Service.DeleteMemo(ctx, userID, memoID); err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})

	g.DELETE("/memo/:memoId/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("Memo ID is not a number: %s", c.Param("memoId")), err)
		}
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("Resource ID is not a number: %s", c.Param("resourceId")), err)
		}

		if err := s.Service.UnbindMemoResource(ctx, userID, memoID, resourceID); err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

