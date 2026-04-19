package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"

	"github.com/gorilla/sessions"
)

var (
	userIDContextKey = "user-id"
	sessionName      = "memos_session"
)

func getUserIDContextKey() string {
	return userIDContextKey
}

func setUserSession(ctx Context, user *api.User) error {
	sess, _ := getSession(sessionName, ctx)
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 24 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	sess.Values[userIDContextKey] = user.ID
	err := sess.Save(ctx.Request(), ctx.Writer())
	if err != nil {
		return fmt.Errorf("failed to set session, err: %w", err)
	}
	return nil
}

func removeUserSession(ctx Context) error {
	sess, _ := getSession(sessionName, ctx)
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
	}
	sess.Values[userIDContextKey] = nil
	err := sess.Save(ctx.Request(), ctx.Writer())
	if err != nil {
		return fmt.Errorf("failed to set session, err: %w", err)
	}
	return nil
}

func aclMiddleware(s *Server) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			ctx := c.Request().Context()
			path := c.Path()

			if s.DefaultAuthSkipper(c) {
				return next(c)
			}

			sess, _ := getSession(sessionName, c)
			userIDValue := sess.Values[userIDContextKey]
			if userIDValue != nil {
				userID, _ := strconv.Atoi(fmt.Sprintf("%v", userIDValue))
				userFind := &api.UserFind{
					ID: &userID,
				}
				user, err := s.Store.FindUser(ctx, userFind)
				if err != nil && common.ErrorCode(err) != common.NotFound {
					return internalError(fmt.Sprintf("Failed to find user by ID: %d", userID), err)
				}
				if user != nil {
					if user.RowStatus == api.Archived {
						return forbiddenError(fmt.Sprintf("User has been archived with username %s", user.Username))
					}
					c.Set(getUserIDContextKey(), userID)
				}
			}

			if common.HasPrefixes(path, "/api/ping", "/api/status", "/api/user/:id", "/api/memo") && c.Request().Method == http.MethodGet {
				return next(c)
			}

			userID := c.Get(getUserIDContextKey())
			if userID == nil {
				return unauthorizedError("Missing user in session")
			}

			return next(c)
		}
	}
}
