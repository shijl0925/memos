package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	metric "github.com/usememos/memos/plugin/metrics"
)

func (s *Server) registerMemoRoutes(g Group) {
	g.POST("/memo", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		memoCreate := &api.MemoCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoCreate); err != nil {
			return badRequestError("Malformatted post memo request", err)
		}

		if memoCreate.Visibility == "" {
			userSettingMemoVisibilityKey := api.UserSettingMemoVisibilityKey
			userMemoVisibilitySetting, err := s.Store.FindUserSetting(ctx, &api.UserSettingFind{
				UserID: userID,
				Key:    &userSettingMemoVisibilityKey,
			})
			if err != nil {
				return internalError("Failed to find user setting", err)
			}

			if userMemoVisibilitySetting != nil {
				memoVisibility := api.Private
				err := json.Unmarshal([]byte(userMemoVisibilitySetting.Value), &memoVisibility)
				if err != nil {
					return internalError("Failed to unmarshal user setting value", err)
				}
				memoCreate.Visibility = memoVisibility
			} else {
				// Private is the default memo visibility.
				memoCreate.Visibility = api.Private
			}
		}

		memoCreate.CreatorID = userID
		memo, err := s.Store.CreateMemo(ctx, memoCreate)
		if err != nil {
			return internalError("Failed to create memo", err)
		}
		if err := s.createMemoCreateActivity(c, memo); err != nil {
			return internalError("Failed to create activity", err)
		}

		for _, resourceID := range memoCreate.ResourceIDList {
			if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
				MemoID:     memo.ID,
				ResourceID: resourceID,
			}); err != nil {
				return internalError("Failed to upsert memo resource", err)
			}
		}

		memo, err = s.Store.ComposeMemo(ctx, memo)
		if err != nil {
			return internalError("Failed to compose memo", err)
		}

		if err := writeJSON(c, memo); err != nil {
			return internalError("Failed to encode memo response", err)
		}
		return nil
	})

	g.PATCH("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{
			ID: &memoID,
		})
		if err != nil {
			return internalError("Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		currentTs := time.Now().Unix()
		memoPatch := &api.MemoPatch{
			ID:        memoID,
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(memoPatch); err != nil {
			return badRequestError("Malformatted patch memo request", err)
		}

		memo, err = s.Store.PatchMemo(ctx, memoPatch)
		if err != nil {
			return internalError("Failed to patch memo", err)
		}

		for _, resourceID := range memoPatch.ResourceIDList {
			if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
				MemoID:     memo.ID,
				ResourceID: resourceID,
			}); err != nil {
				return internalError("Failed to upsert memo resource", err)
			}
		}

		memo, err = s.Store.ComposeMemo(ctx, memo)
		if err != nil {
			return internalError("Failed to compose memo", err)
		}

		if err := writeJSON(c, memo); err != nil {
			return internalError("Failed to encode memo response", err)
		}
		return nil
	})

	g.GET("/memo", func(c Context) error {
		ctx := c.Request().Context()
		memoFind := &api.MemoFind{}
		if userID, err := strconv.Atoi(c.QueryParam("creatorId")); err == nil {
			memoFind.CreatorID = &userID
		}

		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			if memoFind.CreatorID == nil {
				return badRequestError("Missing user id to find memo", nil)
			}
			memoFind.VisibilityList = []api.Visibility{api.Public}
		} else {
			if memoFind.CreatorID == nil {
				memoFind.CreatorID = &currentUserID
			} else {
				memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected}
			}
		}

		rowStatus := api.RowStatus(c.QueryParam("rowStatus"))
		if rowStatus != "" {
			memoFind.RowStatus = &rowStatus
		}
		pinnedStr := c.QueryParam("pinned")
		if pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		tag := c.QueryParam("tag")
		if tag != "" {
			contentSearch := "#" + tag
			memoFind.ContentSearch = &contentSearch
		}
		visibilityListStr := c.QueryParam("visibility")
		if visibilityListStr != "" {
			visibilityList := []api.Visibility{}
			for _, visibility := range strings.Split(visibilityListStr, ",") {
				visibilityList = append(visibilityList, api.Visibility(visibility))
			}
			memoFind.VisibilityList = visibilityList
		}
		if limit, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
			memoFind.Limit = limit
		}
		if offset, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
			memoFind.Offset = offset
		}

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return internalError("Failed to fetch memo list", err)
		}

		var pinnedMemoList []*api.Memo
		var unpinnedMemoList []*api.Memo

		for _, memo := range list {
			if memo.Pinned {
				pinnedMemoList = append(pinnedMemoList, memo)
			} else {
				unpinnedMemoList = append(unpinnedMemoList, memo)
			}
		}

		sort.Slice(pinnedMemoList, func(i, j int) bool {
			return pinnedMemoList[i].DisplayTs > pinnedMemoList[j].DisplayTs
		})
		sort.Slice(unpinnedMemoList, func(i, j int) bool {
			return unpinnedMemoList[i].DisplayTs > unpinnedMemoList[j].DisplayTs
		})

		memoList := []*api.Memo{}
		memoList = append(memoList, pinnedMemoList...)
		memoList = append(memoList, unpinnedMemoList...)

		if memoFind.Limit != 0 {
			memoList = memoList[memoFind.Offset:common.Min(len(memoList), memoFind.Offset+memoFind.Limit)]
		}

		if err := writeJSON(c, memoList); err != nil {
			return internalError("Failed to encode memo list response", err)
		}
		return nil
	})

	g.GET("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		memoFind := &api.MemoFind{
			ID: &memoID,
		}
		memo, err := s.Store.FindMemo(ctx, memoFind)
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Memo ID not found: %d", memoID), err)
			}

			return internalError(fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if memo.Visibility == api.Private {
			if !ok || memo.CreatorID != userID {
				return forbiddenError("this memo is private only")
			}
		} else if memo.Visibility == api.Protected {
			if !ok {
				return forbiddenError("this memo is protected, missing user in session")
			}
		}

		if err := writeJSON(c, memo); err != nil {
			return internalError("Failed to encode memo response", err)
		}
		return nil
	})

	g.POST("/memo/:memoId/organizer", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		memoOrganizerUpsert := &api.MemoOrganizerUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoOrganizerUpsert); err != nil {
			return badRequestError("Malformatted post memo organizer request", err)
		}
		memoOrganizerUpsert.MemoID = memoID
		memoOrganizerUpsert.UserID = userID

		err = s.Store.UpsertMemoOrganizer(ctx, memoOrganizerUpsert)
		if err != nil {
			return internalError("Failed to upsert memo organizer", err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{
			ID: &memoID,
		})
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Memo ID not found: %d", memoID), err)
			}

			return internalError(fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
		}

		if err := writeJSON(c, memo); err != nil {
			return internalError("Failed to encode memo response", err)
		}
		return nil
	})

	g.POST("/memo/:memoId/resource", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		memoResourceUpsert := &api.MemoResourceUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(memoResourceUpsert); err != nil {
			return badRequestError("Malformatted post memo resource request", err)
		}
		resourceFind := &api.ResourceFind{
			ID: &memoResourceUpsert.ResourceID,
		}
		resource, err := s.Store.FindResource(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to fetch resource", err)
		}
		if resource == nil {
			return badRequestError("Resource not found", nil)
		} else if resource.CreatorID != userID {
			return unauthorizedError("Unauthorized to bind this resource")
		}

		memoResourceUpsert.MemoID = memoID
		currentTs := time.Now().Unix()
		memoResourceUpsert.UpdatedTs = &currentTs
		if _, err := s.Store.UpsertMemoResource(ctx, memoResourceUpsert); err != nil {
			return internalError("Failed to upsert memo resource", err)
		}

		if err := writeJSON(c, resource); err != nil {
			return internalError("Failed to encode resource response", err)
		}
		return nil
	})

	g.GET("/memo/:memoId/resource", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		resourceFind := &api.ResourceFind{
			MemoID: &memoID,
		}
		resourceList, err := s.Store.FindResourceList(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to fetch resource list", err)
		}

		if err := writeJSON(c, resourceList); err != nil {
			return internalError("Failed to encode resource list response", err)
		}
		return nil
	})

	g.GET("/memo/amount", func(c Context) error {
		ctx := c.Request().Context()
		normalRowStatus := api.Normal
		memoFind := &api.MemoFind{
			RowStatus: &normalRowStatus,
		}
		if userID, err := strconv.Atoi(c.QueryParam("userId")); err == nil {
			memoFind.CreatorID = &userID
		}

		memoList, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return internalError("Failed to find memo list", err)
		}

		if err := writeJSON(c, len(memoList)); err != nil {
			return internalError("Failed to encode memo amount", err)
		}
		return nil
	})

	g.GET("/memo/stats", func(c Context) error {
		ctx := c.Request().Context()
		normalStatus := api.Normal
		memoFind := &api.MemoFind{
			RowStatus: &normalStatus,
		}
		if creatorID, err := strconv.Atoi(c.QueryParam("creatorId")); err == nil {
			memoFind.CreatorID = &creatorID
		}
		if memoFind.CreatorID == nil {
			return badRequestError("Missing user id to find memo", nil)
		}

		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			memoFind.VisibilityList = []api.Visibility{api.Public}
		} else {
			if *memoFind.CreatorID != currentUserID {
				memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected}
			} else {
				memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected, api.Private}
			}
		}

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return internalError("Failed to fetch memo list", err)
		}

		displayTsList := []int64{}
		for _, memo := range list {
			displayTsList = append(displayTsList, memo.DisplayTs)
		}

		if err := writeJSON(c, displayTsList); err != nil {
			return internalError("Failed to encode memo stats response", err)
		}
		return nil
	})

	g.GET("/memo/all", func(c Context) error {
		ctx := c.Request().Context()
		memoFind := &api.MemoFind{}

		_, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			memoFind.VisibilityList = []api.Visibility{api.Public}
		} else {
			memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected}
		}

		pinnedStr := c.QueryParam("pinned")
		if pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		tag := c.QueryParam("tag")
		if tag != "" {
			contentSearch := "#" + tag + " "
			memoFind.ContentSearch = &contentSearch
		}
		visibilityListStr := c.QueryParam("visibility")
		if visibilityListStr != "" {
			visibilityList := []api.Visibility{}
			for _, visibility := range strings.Split(visibilityListStr, ",") {
				visibilityList = append(visibilityList, api.Visibility(visibility))
			}
			memoFind.VisibilityList = visibilityList
		}
		if limit, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
			memoFind.Limit = limit
		}
		if offset, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
			memoFind.Offset = offset
		}

		// Only fetch normal status memos.
		normalStatus := api.Normal
		memoFind.RowStatus = &normalStatus

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return internalError("Failed to fetch all memo list", err)
		}

		sort.Slice(list, func(i, j int) bool {
			return list[i].DisplayTs > list[j].DisplayTs
		})

		if memoFind.Limit != 0 {
			list = list[memoFind.Offset:common.Min(len(list), memoFind.Offset+memoFind.Limit)]
		}

		if err := writeJSON(c, list); err != nil {
			return internalError("Failed to encode all memo list response", err)
		}
		return nil
	})

	g.DELETE("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{
			ID: &memoID,
		})
		if err != nil {
			return internalError("Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		memoDelete := &api.MemoDelete{
			ID: memoID,
		}
		if err := s.Store.DeleteMemo(ctx, memoDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Memo ID not found: %d", memoID), nil)
			}
			return internalError(fmt.Sprintf("Failed to delete memo ID: %v", memoID), err)
		}

		return c.JSON(http.StatusOK, true)
	})

	g.DELETE("/memo/:memoId/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("Memo ID is not a number: %s", c.Param("memoId")), err)
		}
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("Resource ID is not a number: %s", c.Param("resourceId")), err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{
			ID: &memoID,
		})
		if err != nil {
			return internalError("Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		memoResourceDelete := &api.MemoResourceDelete{
			MemoID:     &memoID,
			ResourceID: &resourceID,
		}
		if err := s.Store.DeleteMemoResource(ctx, memoResourceDelete); err != nil {
			return internalError("Failed to fetch resource list", err)
		}

		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) createMemoCreateActivity(c Context, memo *api.Memo) error {
	ctx := c.Request().Context()
	payload := api.ActivityMemoCreatePayload{
		Content:    memo.Content,
		Visibility: memo.Visibility.String(),
	}
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: memo.CreatorID,
		Type:      api.ActivityMemoCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Collect(ctx, &metric.Metric{
		Name: string(activity.Type),
	})
	return err
}
