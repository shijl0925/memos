package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"golang.org/x/exp/slices"
)

var tagRegexp = regexp.MustCompile(`#([^\s#]+)`)

// UpsertTag validates the payload, persists the tag and records an activity.
func (s *Service) UpsertTag(ctx context.Context, userID int, upsert *api.TagUpsert) (*api.Tag, error) {
	if upsert.Name == "" {
		return nil, common.Errorf(common.Invalid, fmt.Errorf("tag name shouldn't be empty"))
	}
	upsert.CreatorID = userID
	tag, err := s.Store.UpsertTag(ctx, upsert)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert tag: %w", err)
	}
	if err := s.createTagCreateActivity(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return tag, nil
}

// ListTagNames returns a sorted list of tag names created by the user.
func (s *Service) ListTagNames(ctx context.Context, userID int) ([]string, error) {
	tagList, err := s.Store.FindTagList(ctx, &api.TagFind{CreatorID: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find tag list: %w", err)
	}
	names := make([]string, 0, len(tagList))
	for _, tag := range tagList {
		names = append(names, tag.Name)
	}
	return names, nil
}

// DeleteTag removes a tag by name for the given user.
func (s *Service) DeleteTag(ctx context.Context, userID int, name string) error {
	if name == "" {
		return common.Errorf(common.Invalid, fmt.Errorf("tag name shouldn't be empty"))
	}
	return s.Store.DeleteTag(ctx, &api.TagDelete{Name: name, CreatorID: userID})
}

// GetTagSuggestions scans memo content for tags that are not yet saved and
// returns a deduplicated, sorted list of suggestions.
func (s *Service) GetTagSuggestions(ctx context.Context, userID int) ([]string, error) {
	contentSearch := "#"
	normalRowStatus := api.Normal
	memoList, err := s.Store.FindMemoList(ctx, &api.MemoFind{
		CreatorID:     &userID,
		ContentSearch: &contentSearch,
		RowStatus:     &normalRowStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find memo list: %w", err)
	}

	existTagList, err := s.Store.FindTagList(ctx, &api.TagFind{CreatorID: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find tag list: %w", err)
	}
	existingNames := make([]string, 0, len(existTagList))
	for _, tag := range existTagList {
		existingNames = append(existingNames, tag.Name)
	}

	tagMapSet := make(map[string]bool)
	for _, memo := range memoList {
		for _, tag := range findTagListFromContent(memo.Content) {
			if !slices.Contains(existingNames, tag) {
				tagMapSet[tag] = true
			}
		}
	}

	tagList := make([]string, 0, len(tagMapSet))
	for tag := range tagMapSet {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	return tagList, nil
}

// findTagListFromContent extracts unique tag names from memo content.
func findTagListFromContent(content string) []string {
	tagMapSet := make(map[string]bool)
	matches := tagRegexp.FindAllStringSubmatch(content, -1)
	for _, v := range matches {
		tagMapSet[v[1]] = true
	}
	tagList := make([]string, 0, len(tagMapSet))
	for tag := range tagMapSet {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	return tagList
}
