package server

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
)

func (s *Server) registerTagRoutes(g *gin.RouterGroup) {
	g.GET("/tag", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		contentSearch := "#"
		normalRowStatus := api.Normal
		memoFind := api.MemoFind{
			CreatorID:     &userID,
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
}
