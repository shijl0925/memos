package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

const memoContentLengthOverflowMessage = "Content size overflow, up to 1MB"

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

		if memoCreate.Visibility == "" {
			userMemoVisibilitySetting, err := s.Store.FindUserSetting(ctx, &api.UserSettingFind{
				UserID: userID,
				Key:    api.UserSettingMemoVisibilityKey,
			})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user setting", err)
			}

			if userMemoVisibilitySetting != nil {
				memoVisibility := api.Private
				if err := json.Unmarshal([]byte(userMemoVisibilitySetting.Value), &memoVisibility); err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal user setting value", err)
				}
				memoCreate.Visibility = memoVisibility
			} else {
				memoCreate.Visibility = api.Private
			}
		}

		disablePublicMemosSystemSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
			Name: api.SystemSettingDisablePublicMemosName,
		})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find system setting", err)
		}
		if disablePublicMemosSystemSetting != nil {
			disablePublicMemos := false
			if err := json.Unmarshal([]byte(disablePublicMemosSystemSetting.Value), &disablePublicMemos); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal system setting", err)
			}
			if disablePublicMemos {
				user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
				if err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
				}
				if user.Role == api.NormalUser {
					memoCreate.Visibility = api.Private
				}
			}
		}

		if err := validateMemoContentLength(memoCreate.Content); err != nil {
			return err
		}

		memoCreate.CreatorID = userID
		memo, err := s.Store.CreateMemo(ctx, memoCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create memo", err)
		}
		if err := s.createMemoCreateActivity(c, memo); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}

		for _, resourceID := range memoCreate.ResourceIDList {
			if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
				MemoID:     memo.ID,
				ResourceID: resourceID,
			}); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert memo resource", err)
			}
		}

		memo, err = s.Store.ComposeMemo(ctx, memo)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to compose memo", err)
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

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		currentTs := time.Now().Unix()
		memoPatch := &api.MemoPatch{
			ID:        memoID,
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(memoPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch memo request", err)
		}

		if memoPatch.Content != nil {
			if err := validateMemoContentLength(*memoPatch.Content); err != nil {
				return err
			}
		}

		memo, err = s.Store.PatchMemo(ctx, memoPatch)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to patch memo", err)
		}

		for _, resourceID := range memoPatch.ResourceIDList {
			if _, err := s.Store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
				MemoID:     memo.ID,
				ResourceID: resourceID,
			}); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert memo resource", err)
			}
		}

		memo, err = s.Store.ComposeMemo(ctx, memo)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to compose memo", err)
		}
		return c.JSON(http.StatusOK, composeResponse(memo))
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
				return newHTTPError(http.StatusBadRequest, "Missing user id to find memo")
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

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch memo list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	})

	g.GET("/memo/:memoId", func(c Context) error {
		ctx := c.Request().Context()
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPErrorWithInternal(http.StatusNotFound, fmt.Sprintf("Memo ID not found: %d", memoID), err)
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if memo.Visibility == api.Private {
			if !ok || memo.CreatorID != userID {
				return newHTTPError(http.StatusForbidden, "this memo is private only")
			}
		} else if memo.Visibility == api.Protected && !ok {
			return newHTTPError(http.StatusForbidden, "this memo is protected, missing user in session")
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
		memoOrganizerUpsert.MemoID = memoID
		memoOrganizerUpsert.UserID = userID

		if err := s.Store.UpsertMemoOrganizer(ctx, memoOrganizerUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert memo organizer", err)
		}

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPErrorWithInternal(http.StatusNotFound, fmt.Sprintf("Memo ID not found: %d", memoID), err)
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
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
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &memoResourceUpsert.ResourceID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource", err)
		}
		if resource == nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Resource not found", err)
		}
		if resource.CreatorID != userID {
			return newHTTPErrorWithInternal(http.StatusUnauthorized, "Unauthorized to bind this resource", err)
		}

		memoResourceUpsert.MemoID = memoID
		currentTs := time.Now().Unix()
		memoResourceUpsert.UpdatedTs = &currentTs
		if _, err := s.Store.UpsertMemoResource(ctx, memoResourceUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert memo resource", err)
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
		normalStatus := api.Normal
		memoFind := &api.MemoFind{RowStatus: &normalStatus}
		if creatorID, err := strconv.Atoi(c.QueryParam("creatorId")); err == nil {
			memoFind.CreatorID = &creatorID
		}
		if memoFind.CreatorID == nil {
			return newHTTPError(http.StatusBadRequest, "Missing user id to find memo")
		}

		currentUserID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			memoFind.VisibilityList = []api.Visibility{api.Public}
		} else if *memoFind.CreatorID != currentUserID {
			memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected}
		} else {
			memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected, api.Private}
		}

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch memo list", err)
		}

		createdTsList := []int64{}
		for _, memo := range list {
			createdTsList = append(createdTsList, memo.CreatedTs)
		}
		return c.JSON(http.StatusOK, composeResponse(createdTsList))
	})

	g.GET("/memo/all", func(c Context) error {
		ctx := c.Request().Context()
		memoFind := &api.MemoFind{}

		if _, ok := c.Get(getUserIDContextKey()).(int); !ok {
			memoFind.VisibilityList = []api.Visibility{api.Public}
		} else {
			memoFind.VisibilityList = []api.Visibility{api.Public, api.Protected}
		}

		if pinnedStr := c.QueryParam("pinned"); pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		if tag := c.QueryParam("tag"); tag != "" {
			contentSearch := "#" + tag + " "
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

		normalStatus := api.Normal
		memoFind.RowStatus = &normalStatus

		list, err := s.Store.FindMemoList(ctx, memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch all memo list", err)
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

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		if err := s.Store.DeleteMemo(ctx, &api.MemoDelete{ID: memoID}); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Memo ID not found: %d", memoID))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to delete memo ID: %v", memoID), err)
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

		memo, err := s.Store.FindMemo(ctx, &api.MemoFind{ID: &memoID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo", err)
		}
		if memo.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		if err := s.Store.DeleteMemoResource(ctx, &api.MemoResourceDelete{
			MemoID:     &memoID,
			ResourceID: &resourceID,
		}); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource list", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func validateMemoContentLength(content string) error {
	if len(content) > api.MaxContentLength {
		return newHTTPError(http.StatusBadRequest, memoContentLengthOverflowMessage)
	}
	return nil
}

func (s *Server) createMemoCreateActivity(c Context, memo *api.Memo) error {
	ctx := c.Request().Context()
	payload := api.ActivityMemoCreatePayload{
		Content:    memo.Content,
		Visibility: memo.Visibility.String(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: memo.CreatorID,
		Type:      api.ActivityMemoCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
