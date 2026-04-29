package server

import (
	"encoding/json"
	ninja "github.com/shijl0925/gin-ninja"
	"net/http"

	"github.com/usememos/memos/api"
)

func (s *Server) registerAuthRoutes(r *ninja.Router, secret string) {
	ninja.Post(r, "/auth/signin", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/auth/signin/sso", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/auth/signup", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/auth/signout", adaptNinjaHandler(func(c Context) error {
		RemoveTokensAndCookies(c)
		return c.JSON(http.StatusOK, true)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}
