package server

import (
	"net/http"

	"github.com/usememos/memos/api"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerSystemRoutes(g *gin.RouterGroup) {
	g.GET("/ping", func(c *gin.Context) {
		data := s.Profile
		writeJSON(c, data)
	})

	g.GET("/status", func(c *gin.Context) {
		ownerUserType := api.Owner
		ownerUserFind := api.UserFind{
			Role: &ownerUserType,
		}
		ownerUser, err := s.Store.FindUser(&ownerUserFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find owner user", err)
			return
		}

		if ownerUser != nil {
			// data desensitize
			ownerUser.OpenID = ""
		}

		systemStatus := api.SystemStatus{
			Owner:   ownerUser,
			Profile: s.Profile,
		}
		writeJSON(c, systemStatus)
	})
}
