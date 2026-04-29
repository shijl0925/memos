package server

import (
	ninja "github.com/shijl0925/gin-ninja"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerRSSRoutes(r *ninja.Router) {
	ninja.Get(r, "/explore/rss.xml", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		systemCustomizedProfile, err := s.Service.GetSystemCustomizedProfile(ctx)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to get system customized profile", err)
		}

		normalStatus := api.Normal
		memoFind := api.MemoFind{RowStatus: &normalStatus, VisibilityList: []api.Visibility{api.Public}}
		memoList, err := s.Store.FindMemoList(ctx, &memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo list", err)
		}

		baseURL := c.Scheme() + "://" + c.Request().Host
		rss, err := generateRSSFromMemoList(memoList, baseURL, systemCustomizedProfile)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate rss", err)
		}
		c.Header(headerContentType, mimeApplicationXMLCharset)
		return c.String(http.StatusOK, rss)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/u/:id/rss.xml", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		systemCustomizedProfile, err := s.Service.GetSystemCustomizedProfile(ctx)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to get system customized profile", err)
		}

		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "User id is not a number", err)
		}

		normalStatus := api.Normal
		memoFind := api.MemoFind{CreatorID: &id, RowStatus: &normalStatus, VisibilityList: []api.Visibility{api.Public}}
		memoList, err := s.Store.FindMemoList(ctx, &memoFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo list", err)
		}

		baseURL := c.Scheme() + "://" + c.Request().Host
		rss, err := generateRSSFromMemoList(memoList, baseURL, systemCustomizedProfile)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate rss", err)
		}
		c.Header(headerContentType, mimeApplicationXMLCharset)
		return c.String(http.StatusOK, rss)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}

const MaxRSSItemCount = 100
const MaxRSSItemTitleLength = 100

func generateRSSFromMemoList(memoList []*api.Memo, baseURL string, profile *api.CustomizedProfile) (string, error) {
	feed := &feeds.Feed{
		Title:       profile.Name,
		Link:        &feeds.Link{Href: baseURL},
		Description: profile.Description,
		Created:     time.Now(),
	}

	itemCountLimit := common.Min(len(memoList), MaxRSSItemCount)
	feed.Items = make([]*feeds.Item, itemCountLimit)
	for i := 0; i < itemCountLimit; i++ {
		memo := memoList[i]
		feed.Items[i] = &feeds.Item{
			Title:       getRSSItemTitle(memo.Content),
			Link:        &feeds.Link{Href: baseURL + "/m/" + strconv.Itoa(memo.ID)},
			Description: getRSSItemDescription(memo.Content),
			Created:     time.Unix(memo.CreatedTs, 0),
		}
	}

	rss, err := feed.ToRss()
	if err != nil {
		return "", err
	}
	return rss, nil
}

func getRSSItemTitle(content string) string {
	var title string
	if isTitleDefined(content) {
		title = strings.Split(content, "\n")[0][2:]
	} else {
		title = strings.Split(content, "\n")[0]
		titleLengthLimit := common.Min(len(title), MaxRSSItemTitleLength)
		if titleLengthLimit < len(title) {
			title = title[:titleLengthLimit] + "..."
		}
	}
	return title
}

func getRSSItemDescription(content string) string {
	var description string
	if isTitleDefined(content) {
		firstLineEnd := strings.Index(content, "\n")
		if firstLineEnd == -1 {
			description = ""
		} else {
			description = content[firstLineEnd+1:]
		}
	} else {
		description = content
	}
	return description
}

func isTitleDefined(content string) bool {
	return strings.HasPrefix(content, "# ")
}
