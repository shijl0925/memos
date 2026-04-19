package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
			return unauthorizedError("Missing user in session")
		}

		tagUpsert := &api.TagUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(tagUpsert); err != nil {
			return badRequestError("Malformatted post tag request", err)
		}
		if tagUpsert.Name == "" {
			return badRequestError("Tag name should not be empty", nil)
		}

		tagUpsert.CreatorID = userID
		tag, err := s.Store.UpsertTag(ctx, tagUpsert)
		if err != nil {
			return internalError("Failed to upsert tag", err)
		}
		if err := s.createTagCreateActivity(c, tag); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, tag.Name); err != nil {
			return internalError("Failed to encode tag response", err)
		}
		return nil
	})

	g.GET("/tag", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return badRequestError("Missing user id to find tag", nil)
		}

		tagFind := &api.TagFind{
			CreatorID: userID,
		}
		tagList, err := s.Store.FindTagList(ctx, tagFind)
		if err != nil {
			return internalError("Failed to find tag list", err)
		}

		tagNameList := []string{}
		for _, tag := range tagList {
			tagNameList = append(tagNameList, tag.Name)
		}

		if err := writeJSON(c, tagNameList); err != nil {
			return internalError("Failed to encode tags response", err)
		}
		return nil
	})

	g.GET("/tag/suggestion", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return badRequestError("Missing user session", nil)
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
			return internalError("Failed to find memo list", err)
		}

		tagFind := &api.TagFind{
			CreatorID: userID,
		}
		existTagList, err := s.Store.FindTagList(ctx, tagFind)
		if err != nil {
			return internalError("Failed to find tag list", err)
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

		if err := writeJSON(c, tagList); err != nil {
			return internalError("Failed to encode tags response", err)
		}
		return nil
	})

	g.DELETE("/tag/:tagName", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		tagName, err := url.QueryUnescape(c.Param("tagName"))
		if err != nil {
			return badRequestError("Invalid tag name", err)
		} else if tagName == "" {
			return badRequestError("Tag name should not be empty", nil)
		}

		tagDelete := &api.TagDelete{
			Name:      tagName,
			CreatorID: userID,
		}
		if err := s.Store.DeleteTag(ctx, tagDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Tag name not found: %s", tagName), nil)
			}
			return internalError(fmt.Sprintf("Failed to delete tag name: %v", tagName), err)
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
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: tag.CreatorID,
		Type:      api.ActivityTagCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
