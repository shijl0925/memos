package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
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
	if err := sess.Save(ctx.Request(), ctx.Writer()); err != nil {
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
	if err := sess.Save(ctx.Request(), ctx.Writer()); err != nil {
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
				user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
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

			if common.HasPrefixes(path, "/api/ping", "/api/status", "/api/idp", "/api/user/:id", "/api/memo") && c.Request().Method == http.MethodGet {
				return next(c)
			}

			if c.Get(getUserIDContextKey()) == nil {
				return unauthorizedError("Missing user in session")
			}

			return next(c)
		}
	}
}
