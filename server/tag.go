package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"golang.org/x/exp/slices"
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
		if tagUpsert.Name == "" {
			return newHTTPError(http.StatusBadRequest, "Tag name shouldn't be empty")
		}

		tagUpsert.CreatorID = userID
		tag, err := s.Store.UpsertTag(ctx, tagUpsert)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert tag", err)
		}
		if err := s.createTagCreateActivity(c, tag); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(tag.Name))
	})

	g.GET("/tag", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusBadRequest, "Missing user id to find tag")
		}

		tagFind := &api.TagFind{CreatorID: userID}
		tagList, err := s.Store.FindTagList(ctx, tagFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find tag list", err)
		}

		tagNameList := []string{}
		for _, tag := range tagList {
			tagNameList = append(tagNameList, tag.Name)
		}
		return c.JSON(http.StatusOK, composeResponse(tagNameList))
	})

	g.GET("/tag/suggestion", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusBadRequest, "Missing user session")
		}
		contentSearch := "#"
		normalRowStatus := api.Normal
		memoFind := api.MemoFind{
			CreatorID:     &userID,
			ContentSearch: &contentSearch,
			RowStatus:     &normalRowStatus,
		}

		memoList, err := s.Store.FindMemoList(ctx, &memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo list", err)
		}

		existTagList, err := s.Store.FindTagList(ctx, &api.TagFind{CreatorID: userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find tag list", err)
		}
		tagNameList := []string{}
		for _, tag := range existTagList {
			tagNameList = append(tagNameList, tag.Name)
		}

		tagMapSet := make(map[string]bool)
		for _, memo := range memoList {
			for _, tag := range findTagListFromMemoContent(memo.Content) {
				if !slices.Contains(tagNameList, tag) {
					tagMapSet[tag] = true
				}
			}
		}
		tagList := []string{}
		for tag := range tagMapSet {
			tagList = append(tagList, tag)
		}
		sort.Strings(tagList)
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
		if tagDelete.Name == "" {
			return newHTTPError(http.StatusBadRequest, "Tag name shouldn't be empty")
		}

		tagDelete.CreatorID = userID
		if err := s.Store.DeleteTag(ctx, tagDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Tag name not found: %s", tagDelete.Name))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to delete tag name: %v", tagDelete.Name), err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

var tagRegexp = regexp.MustCompile(`#([^\s#]+)`)

func findTagListFromMemoContent(memoContent string) []string {
	tagMapSet := make(map[string]bool)
	matches := tagRegexp.FindAllStringSubmatch(memoContent, -1)
	for _, v := range matches {
		tagName := v[1]
		tagMapSet[tagName] = true
	}

	tagList := []string{}
	for tag := range tagMapSet {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	return tagList
}

func (s *Server) createTagCreateActivity(c Context, tag *api.Tag) error {
	ctx := c.Request().Context()
	payload := api.ActivityTagCreatePayload{
		TagName: tag.Name,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: tag.CreatorID,
		Type:      api.ActivityTagCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
