package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerMemoRoutes(g *gin.RouterGroup) {
	g.POST("/memo", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		memoCreate := &api.MemoCreate{
			CreatorID: userID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(memoCreate); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted post memo request", err)
			return
		}

		memo, err := s.Store.CreateMemo(memoCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create memo", err)
			return
		}
		writeJSON(c, memo)
	})

	g.PATCH("/memo/:memoId", func(c *gin.Context) {
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
			return
		}

		memoPatch := &api.MemoPatch{
			ID: memoID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(memoPatch); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted patch memo request", err)
			return
		}

		memo, err := s.Store.PatchMemo(memoPatch)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to patch memo", err)
			return
		}
		writeJSON(c, memo)
	})

	g.GET("/memo", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		memoFind := &api.MemoFind{
			CreatorID: &userID,
		}

		rowStatus := api.RowStatus(c.Query("rowStatus"))
		if rowStatus != "" {
			memoFind.RowStatus = &rowStatus
		}
		pinnedStr := c.Query("pinned")
		if pinnedStr != "" {
			pinned := pinnedStr == "true"
			memoFind.Pinned = &pinned
		}
		tag := c.Query("tag")
		if tag != "" {
			contentSearch := "#" + tag + " "
			memoFind.ContentSearch = &contentSearch
		}
		if limit, err := strconv.Atoi(c.Query("limit")); err == nil {
			memoFind.Limit = limit
		}
		if offset, err := strconv.Atoi(c.Query("offset")); err == nil {
			memoFind.Offset = offset
		}

		list, err := s.Store.FindMemoList(memoFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch memo list", err)
			return
		}
		writeJSON(c, list)
	})

	g.POST("/memo/:memoId/organizer", func(c *gin.Context) {
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
			return
		}

		userID := getCurrentUserID(c)
		memoOrganizerUpsert := &api.MemoOrganizerUpsert{
			MemoID: memoID,
			UserID: userID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(memoOrganizerUpsert); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted post memo organizer request", err)
			return
		}

		err = s.Store.UpsertMemoOrganizer(memoOrganizerUpsert)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to upsert memo organizer", err)
			return
		}

		memo, err := s.Store.FindMemo(&api.MemoFind{
			ID: &memoID,
		})
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				abortWithError(c, http.StatusNotFound, fmt.Sprintf("Memo ID not found: %d", memoID), err)
				return
			}

			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
			return
		}
		writeJSON(c, memo)
	})

	g.GET("/memo/:memoId", func(c *gin.Context) {
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
			return
		}

		memoFind := &api.MemoFind{
			ID: &memoID,
		}
		memo, err := s.Store.FindMemo(memoFind)
		if err != nil {
			if common.ErrorCode(err) == common.NotFound {
				abortWithError(c, http.StatusNotFound, fmt.Sprintf("Memo ID not found: %d", memoID), err)
				return
			}

			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to find memo by ID: %v", memoID), err)
			return
		}
		writeJSON(c, memo)
	})

	g.DELETE("/memo/:memoId", func(c *gin.Context) {
		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("memoId")), err)
			return
		}

		memoDelete := &api.MemoDelete{
			ID: memoID,
		}

		err = s.Store.DeleteMemo(memoDelete)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete memo ID: %v", memoID), err)
			return
		}

		c.JSON(http.StatusOK, true)
	})

	g.GET("/memo/amount", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		normalRowStatus := api.Normal
		memoFind := &api.MemoFind{
			CreatorID: &userID,
			RowStatus: &normalRowStatus,
		}

		memoList, err := s.Store.FindMemoList(memoFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find memo list", err)
			return
		}
		writeJSON(c, len(memoList))
	})
}
