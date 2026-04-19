package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) registerAuthRoutes(g *gin.RouterGroup) {
	g.POST("/auth/login", func(c *gin.Context) {
		login := &api.Login{}
		if err := json.NewDecoder(c.Request.Body).Decode(login); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted login request", err)
			return
		}

		userFind := &api.UserFind{
			Email: &login.Email,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to find user by email %s", login.Email), err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusUnauthorized, fmt.Sprintf("User not found with email %s", login.Email), nil)
			return
		} else if user.RowStatus == api.Archived {
			abortWithError(c, http.StatusForbidden, fmt.Sprintf("User has been archived with email %s", login.Email), nil)
			return
		}

		// Compare the stored hashed password, with the hashed version of the password that was received.
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(login.Password)); err != nil {
			// If the two passwords don't match, return a 401 status.
			abortWithError(c, http.StatusUnauthorized, "Incorrect password", err)
			return
		}

		if err = setUserSession(c, user); err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to set login session", err)
			return
		}
		writeJSON(c, user)
	})

	g.POST("/auth/logout", func(c *gin.Context) {
		err := removeUserSession(c)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to set logout session", err)
			return
		}

		c.Status(http.StatusOK)
	})

	g.POST("/auth/signup", func(c *gin.Context) {
		// Don't allow to signup by this api if site owner existed.
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
			abortWithError(c, http.StatusUnauthorized, "Site Owner existed, please contact the site owner to signin account firstly.", err)
			return
		}

		signup := &api.Signup{}
		if err := json.NewDecoder(c.Request.Body).Decode(signup); err != nil {
			abortWithError(c, http.StatusBadRequest, "Malformatted signup request", err)
			return
		}

		// Validate signup form.
		// We can do stricter checks later.
		if len(signup.Email) < 6 {
			abortWithError(c, http.StatusBadRequest, "Email is too short, minimum length is 6.", nil)
			return
		}
		if len(signup.Password) < 6 {
			abortWithError(c, http.StatusBadRequest, "Password is too short, minimum length is 6.", nil)
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to generate password hash", err)
			return
		}

		userCreate := &api.UserCreate{
			Email:        signup.Email,
			Role:         api.Role(signup.Role),
			Name:         signup.Name,
			PasswordHash: string(passwordHash),
			OpenID:       common.GenUUID(),
		}
		user, err := s.Store.CreateUser(userCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create user", err)
			return
		}

		err = setUserSession(c, user)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to set signup session", err)
			return
		}
		writeJSON(c, user)
	})
}
