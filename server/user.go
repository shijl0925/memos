package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerUserRoutes(g *gin.RouterGroup) {
	g.POST("/user", func(c *gin.Context) {
		userCreate := &api.UserCreate{}
		if err := json.NewDecoder(c.Request.Body).Decode(userCreate); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted post user request", err)
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to generate password hash", err)
			return
		}

		userCreate.PasswordHash = string(passwordHash)
		user, err := s.Store.CreateUser(userCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create user", err)
			return
		}
		writeJSON(c, user)
	})

	g.GET("/user", func(c *gin.Context) {
		userList, err := s.Store.FindUserList(&api.UserFind{})
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch user list", err)
			return
		}
		writeJSON(c, userList)
	})

	// GET /api/user/me is used to check if the user is logged in.
	g.GET("/user/me", func(c *gin.Context) {
		userSessionID, exists := c.Get(getUserIDContextKey())
		if !exists {
			abortWithError(c, http.StatusUnauthorized, "Missing auth session", nil)
			return
		}

		userID := userSessionID.(int)
		userFind := &api.UserFind{
			ID: &userID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch user", err)
			return
		}
		writeJSON(c, user)
	})

	g.PATCH("/user/me", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		userPatch := &api.UserPatch{
			ID: userID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(userPatch); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted patch user request", err)
			return
		}

		if userPatch.Password != nil && *userPatch.Password != "" {
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(*userPatch.Password), bcrypt.DefaultCost)
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Failed to generate password hash", err)
				return
			}

			passwordHashStr := string(passwordHash)
			userPatch.PasswordHash = &passwordHashStr
		}

		if userPatch.ResetOpenID != nil && *userPatch.ResetOpenID {
			openID := common.GenUUID()
			userPatch.OpenID = &openID
		}

		user, err := s.Store.PatchUser(userPatch)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to patch user", err)
			return
		}
		writeJSON(c, user)
	})

	g.PATCH("/user/:userId", func(c *gin.Context) {
		currentUserID := getCurrentUserID(c)
		currentUser, err := s.Store.FindUser(&api.UserFind{
			ID: &currentUserID,
		})
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to find user", err)
			return
		}
		if currentUser == nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("Current session user not found with ID: %d", currentUserID), nil)
			return
		} else if currentUser.Role != api.Owner {
			abortWithError(c, http.StatusForbidden, "Access forbidden for current session user", nil)
			return
		}

		userID, err := strconv.Atoi(c.Param("userId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("userId")), err)
			return
		}

		userPatch := &api.UserPatch{
			ID: userID,
		}
		if err := json.NewDecoder(c.Request.Body).Decode(userPatch); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted patch user request", err)
			return
		}

		if userPatch.Password != nil && *userPatch.Password != "" {
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(*userPatch.Password), bcrypt.DefaultCost)
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Failed to generate password hash", err)
				return
			}

			passwordHashStr := string(passwordHash)
			userPatch.PasswordHash = &passwordHashStr
		}

		user, err := s.Store.PatchUser(userPatch)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to patch user", err)
			return
		}
		writeJSON(c, user)
	})
}
