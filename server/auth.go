package server

import (
	"encoding/json"
	"net/http"

	"github.com/usememos/memos/api"
)

func (s *Server) registerAuthRoutes(g Group, secret string) {
	g.POST("/auth/signin", func(c Context) error {
		ctx := c.Request().Context()
		signin := &api.SignIn{}
		if err := json.NewDecoder(c.Request().Body).Decode(signin); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signin request", err)
		}

		user, err := s.Service.SignIn(ctx, signin.Username, signin.Password)
		if err != nil {
			return convertServiceError(err)
		}

		if err := GenerateTokensAndSetCookies(c, user, secret); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate tokens", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signin/sso", func(c Context) error {
		ctx := c.Request().Context()
		signin := &api.SSOSignIn{}
		if err := json.NewDecoder(c.Request().Body).Decode(signin); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signin request", err)
		}

		user, err := s.Service.SignInSSO(ctx, signin, getClientIP(c))
		if err != nil {
			return convertServiceError(err)
		}

		if err := GenerateTokensAndSetCookies(c, user, secret); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate tokens", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signup", func(c Context) error {
		ctx := c.Request().Context()
		signup := &api.SignUp{}
		if err := json.NewDecoder(c.Request().Body).Decode(signup); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted signup request", err)
		}

		user, err := s.Service.SignUp(ctx, signup, getClientIP(c))
		if err != nil {
			return convertServiceError(err)
		}

		if err := GenerateTokensAndSetCookies(c, user, secret); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to generate tokens", err)
		}
		return c.JSON(http.StatusOK, composeResponse(user))
	})

	g.POST("/auth/signout", func(c Context) error {
		RemoveTokensAndCookies(c)
		return c.JSON(http.StatusOK, true)
	})
}
