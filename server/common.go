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

func isOpenIDMemoCreateRequest(c Context) bool {
	return c.Request().Method == http.MethodPost && c.Path() == "/api/memo" && c.QueryParam("openId") != ""
}

func (server *Server) DefaultAuthSkipper(c Context) bool {
	path := c.Path()

	if common.HasPrefixes(path, "/api/auth") {
		return true
	}

	if isOpenIDMemoCreateRequest(c) {
		return server.authenticateOpenID(c)
	}

	return false
}

func (server *Server) DefaultCSRFSkipper(c Context) bool {
	return server.DefaultAuthSkipper(c)
}

func (server *Server) authenticateOpenID(c Context) bool {
	ctx := c.Request().Context()
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

// convertServiceError maps a service-layer error to the appropriate HTTP error
// response.  It uses the common.Code embedded in *common.Error to select the
// correct HTTP status code so that handlers stay thin.
func convertServiceError(err error) error {
	switch common.ErrorCode(err) {
	case common.NotAuthorized:
		return newHTTPError(http.StatusUnauthorized, common.ErrorMessage(err))
	case common.NotFound:
		return newHTTPError(http.StatusNotFound, common.ErrorMessage(err))
	case common.Invalid:
		return newHTTPError(http.StatusBadRequest, common.ErrorMessage(err))
	case common.Conflict:
		return newHTTPError(http.StatusConflict, common.ErrorMessage(err))
	default:
		return newHTTPErrorWithInternal(http.StatusInternalServerError, common.ErrorMessage(err), err)
	}
}
