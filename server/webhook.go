package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
)

func (s *Server) registerWebhookRoutes(g *gin.RouterGroup) {
	g.GET("/test", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<strong>Hello, World!</strong>"))
	})

	g.POST("/:openId/memo", func(c *gin.Context) {
		openID := c.Param("openId")
		userFind := &api.UserFind{
			OpenID: &openID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user by open_id", err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusNotFound, fmt.Sprintf("User openId not found: %s", openID), nil)
			return
		}

		memoCreate := &api.MemoCreate{
			CreatorID: user.ID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(memoCreate); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted post memo request by open api", err)
			return
		}

		memo, err := s.Store.CreateMemo(memoCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create memo", err)
			return
		}
		writeJSON(c, memo)
	})

	g.PATCH("/:openId/memo/:memoId", func(c *gin.Context) {
		openID := c.Param("openId")
		userFind := &api.UserFind{
			OpenID: &openID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user by open_id", err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusNotFound, fmt.Sprintf("User openId not found: %s", openID), nil)
			return
		}

		memoID, err := strconv.Atoi(c.Param("memoId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("memoId is not a number: %s", c.Param("memoId")), err)
			return
		}

		memoPatch := &api.MemoPatch{
			ID: memoID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(memoPatch); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted patch memo request by open api", err)
			return
		}

		memo, err := s.Store.PatchMemo(memoPatch)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to patch memo", err)
			return
		}
		writeJSON(c, memo)
	})

	g.GET("/:openId/memo", func(c *gin.Context) {
		openID := c.Param("openId")
		userFind := &api.UserFind{
			OpenID: &openID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user by open_id", err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusNotFound, fmt.Sprintf("Not found user with openid: %s", openID), nil)
			return
		}

		memoFind := &api.MemoFind{
			CreatorID: &user.ID,
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
			contentSearch := tag + " "
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

	g.POST("/:openId/resource", func(c *gin.Context) {
		openID := c.Param("openId")
		userFind := &api.UserFind{
			OpenID: &openID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user by open_id", err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusNotFound, fmt.Sprintf("User openId not found: %s", openID), nil)
			return
		}

		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			abortWithError(c, http.StatusBadRequest, "Upload file overload max size", err)
			return
		}

		file, err := c.FormFile("file")
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Upload file not found", err)
			return
		}

		filename := file.Filename
		filetype := file.Header.Get("Content-Type")
		size := file.Size
		src, err := file.Open()
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to open file", err)
			return
		}
		defer src.Close()

		fileBytes, err := ioutil.ReadAll(src)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to read file", err)
			return
		}

		resourceCreate := &api.ResourceCreate{
			Filename:  filename,
			Type:      filetype,
			Size:      size,
			Blob:      fileBytes,
			CreatorID: user.ID,
		}

		resource, err := s.Store.CreateResource(resourceCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create resource", err)
			return
		}
		writeJSON(c, resource)
	})

	g.GET("/:openId/tag", func(c *gin.Context) {
		openID := c.Param("openId")
		userFind := &api.UserFind{
			OpenID: &openID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user by open_id", err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusNotFound, fmt.Sprintf("User openId not found: %s", openID), nil)
			return
		}

		contentSearch := "#"
		normalRowStatus := api.Normal
		memoFind := api.MemoFind{
			CreatorID:     &user.ID,
			ContentSearch: &contentSearch,
			RowStatus:     &normalRowStatus,
		}

		memoList, err := s.Store.FindMemoList(&memoFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find memo list", err)
			return
		}

		tagMapSet := make(map[string]bool)

		r, err := regexp.Compile("#(.+?) ")
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to compile regexp", err)
			return
		}
		for _, memo := range memoList {
			for _, rawTag := range r.FindAllString(memo.Content, -1) {
				tag := r.ReplaceAllString(rawTag, "$1")
				tagMapSet[tag] = true
			}
		}

		tagList := []string{}
		for tag := range tagMapSet {
			tagList = append(tagList, tag)
		}
		writeJSON(c, tagList)
	})

	g.GET("/r/:resourceId/:filename", func(c *gin.Context) {
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
			return
		}

		filename := c.Param("filename")
		resourceFind := &api.ResourceFind{
			ID:       &resourceID,
			Filename: &filename,
		}
		resource, err := s.Store.FindResource(resourceFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch resource ID: %v", resourceID), err)
			return
		}

		c.Data(http.StatusOK, resource.Type, resource.Blob)
	})
}
