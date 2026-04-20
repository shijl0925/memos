package server

import (
	"net/http"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

type response struct {
	Data any `json:"data"`
}

func composeResponse(data any) response {
	return response{Data: data}
}

func DefaultGetRequestSkipper(c Context) bool {
	return c.Request().Method == http.MethodGet
}

func DefaultAPIRequestSkipper(c Context) bool {
	return common.HasPrefixes(c.Path(), "/api")
}

func (server *Server) DefaultAuthSkipper(c Context) bool {
	ctx := c.Request().Context()
	path := c.Path()

	if common.HasPrefixes(path, "/api/auth") {
		return true
	}

	openID := c.QueryParam("openId")
	if openID != "" {
		user, err := server.Store.FindUser(ctx, &api.UserFind{OpenID: &openID})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return false
		}
		if user != nil {
			c.Set(getUserIDContextKey(), user.ID)
			return true
		}
	}

	return false
}
