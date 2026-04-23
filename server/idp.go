package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerIdentityProviderRoutes(g Group) {
	g.POST("/idp", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		identityProviderCreate := &api.IdentityProviderCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(identityProviderCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post identity provider request", err)
		}

		idp, err := s.Service.CreateIdentityProvider(ctx, userID, identityProviderCreate)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(idp))
	})

	g.PATCH("/idp/:idpId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		idpID, err := strconv.Atoi(c.Param("idpId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("idpId")), err)
		}

		identityProviderPatch := &api.IdentityProviderPatch{ID: idpID}
		if err := json.NewDecoder(c.Request().Body).Decode(identityProviderPatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch identity provider request", err)
		}

		idp, err := s.Service.UpdateIdentityProvider(ctx, userID, idpID, identityProviderPatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(idp))
	})

	g.GET("/idp", func(c Context) error {
		ctx := c.Request().Context()
		var userID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			userID = &id
		}

		idpList, err := s.Service.ListIdentityProviders(ctx, userID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(idpList))
	})

	g.GET("/idp/:idpId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		idpID, err := strconv.Atoi(c.Param("idpId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("idpId")), err)
		}

		idp, err := s.Service.GetIdentityProvider(ctx, userID, idpID)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(idp))
	})

	g.DELETE("/idp/:idpId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		idpID, err := strconv.Atoi(c.Param("idpId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("idpId")), err)
		}

		if err = s.Service.DeleteIdentityProvider(ctx, userID, idpID); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Identity provider ID not found: %d", idpID))
			}
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	})
}
