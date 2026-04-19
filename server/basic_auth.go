package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	userIDContextKey = "user-id"
)

func getUserIDContextKey() string {
	return userIDContextKey
}

func setUserSession(c *gin.Context, user *api.User) error {
	sess := sessions.Default(c)
	sess.Options(sessions.Options{
		Path:     "/",
		MaxAge:   1000 * 3600 * 24 * 30,
		HttpOnly: true,
	})
	sess.Set(userIDContextKey, user.ID)
	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to set session, err: %w", err)
	}
	return nil
}

func removeUserSession(c *gin.Context) error {
	sess := sessions.Default(c)
	sess.Options(sessions.Options{
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
	})
	sess.Delete(userIDContextKey)
	if err := sess.Save(); err != nil {
		return fmt.Errorf("failed to set session, err: %w", err)
	}
	return nil
}

// Use session to store user.id.
func BasicAuthMiddleware(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skips auth
		if common.HasPrefixes(c.Request.URL.Path, "/api/auth", "/api/ping", "/api/status") {
			c.Next()
			return
		}

		sess := sessions.Default(c)
		userIDValue := sess.Get(userIDContextKey)
		if userIDValue == nil {
			abortWithError(c, http.StatusUnauthorized, "Missing userID in session", nil)
			return
		}

		userID, err := strconv.Atoi(fmt.Sprintf("%v", userIDValue))
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to malformatted user id in the session.", err)
			return
		}

		// Even if there is no error, we still need to make sure the user still exists.
		userFind := &api.UserFind{
			ID: &userID,
		}
		user, err := s.Store.FindUser(userFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to find user by ID: %d", userID), err)
			return
		}
		if user == nil {
			abortWithError(c, http.StatusUnauthorized, fmt.Sprintf("Not found user ID: %d", userID), nil)
			return
		} else if user.RowStatus == api.Archived {
			abortWithError(c, http.StatusForbidden, fmt.Sprintf("User has been archived with email %s", user.Email), nil)
			return
		}

		// Stores userID into context.
		c.Set(getUserIDContextKey(), userID)
		c.Next()
	}
}
