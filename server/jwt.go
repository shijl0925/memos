package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	authpkg "github.com/usememos/memos/server/auth"
)

const userIDContextKey = "user-id"

type Claims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

func getUserIDContextKey() string {
	return userIDContextKey
}

func GenerateTokensAndSetCookies(c Context, user *api.User, secret string) error {
	accessToken, err := authpkg.GenerateAccessToken(user.Username, user.ID, secret)
	if err != nil {
		return errors.Wrap(err, "failed to generate access token")
	}

	cookieExp := time.Now().Add(authpkg.CookieExpDuration)
	setTokenCookie(c, authpkg.AccessTokenCookieName, accessToken, cookieExp)

	refreshToken, err := authpkg.GenerateRefreshToken(user.Username, user.ID, secret)
	if err != nil {
		return errors.Wrap(err, "failed to generate refresh token")
	}
	setTokenCookie(c, authpkg.RefreshTokenCookieName, refreshToken, cookieExp)
	return nil
}

func RemoveTokensAndCookies(c Context) {
	cookieExp := time.Now().Add(-1 * time.Hour)
	setTokenCookie(c, authpkg.AccessTokenCookieName, "", cookieExp)
	setTokenCookie(c, authpkg.RefreshTokenCookieName, "", cookieExp)
}

func setTokenCookie(c Context, name, token string, expiration time.Time) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    token,
		Expires:  expiration,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func extractTokenFromHeader(c Context) (string, error) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}
	authHeaderParts := strings.Fields(authHeader)
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return "", errors.New("authorization header format must be Bearer {token}")
	}
	return authHeaderParts[1], nil
}

func findAccessToken(c Context) string {
	accessToken := ""
	cookie, _ := c.Cookie(authpkg.AccessTokenCookieName)
	if cookie != nil {
		accessToken = cookie.Value
	}
	if accessToken == "" {
		accessToken, _ = extractTokenFromHeader(c)
	}
	return accessToken
}

func JWTMiddleware(server *Server, secret string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			path := c.Request().URL.Path
			method := c.Request().Method

			if server.DefaultAuthSkipper(c) {
				return next(c)
			}

			if common.HasPrefixes(path, "/api/ping", "/api/idp", "/api/user/:id") && method == http.MethodGet {
				return next(c)
			}

			token := findAccessToken(c)
			if token == "" {
				if common.HasPrefixes(path, "/o") {
					return next(c)
				}
				if common.HasPrefixes(path, "/api/status", "/api/memo") && method == http.MethodGet {
					return next(c)
				}
				return unauthorizedError("Missing access token")
			}

			claims := &Claims{}
			parsedAccessToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
				if t.Method.Alg() != jwt.SigningMethodHS256.Name {
					return nil, errors.Errorf("unexpected access token signing method=%v, expect %v", t.Header["alg"], jwt.SigningMethodHS256)
				}
				if kid, ok := t.Header["kid"].(string); ok && kid == "v1" {
					return []byte(secret), nil
				}
				return nil, errors.Errorf("unexpected access token kid=%v", t.Header["kid"])
			})
			if !audienceContains(claims.Audience, authpkg.AccessTokenAudienceName) {
				return unauthorizedError(fmt.Sprintf(
					"Invalid access token, audience mismatch, got %q, expected %q. you may send request to the wrong environment",
					claims.Audience, authpkg.AccessTokenAudienceName,
				))
			}

			generateToken := claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) < authpkg.RefreshThresholdDuration
			if err != nil {
				var ve *jwt.ValidationError
				if errors.As(err, &ve) {
					if ve.Errors == jwt.ValidationErrorExpired {
						generateToken = true
					} else {
						return newHTTPErrorWithInternal(http.StatusUnauthorized, "Invalid or expired access token", err)
					}
				} else {
					return newHTTPErrorWithInternal(http.StatusUnauthorized, "Invalid or expired access token", err)
				}
			}

			ctx := c.Request().Context()
			userID, err := strconv.Atoi(claims.Subject)
			if err != nil {
				return unauthorizedError("Malformed ID in the token.")
			}

			user, err := server.Store.FindUser(ctx, &api.UserFind{ID: &userID})
			if err != nil {
				return internalError(fmt.Sprintf("Server error to find user ID: %d", userID), err)
			}
			if user == nil {
				return unauthorizedError(fmt.Sprintf("Failed to find user ID: %d", userID))
			}
			if user.RowStatus == api.Archived {
				return forbiddenError(fmt.Sprintf("User has been archived with username %s", user.Username))
			}

			if generateToken {
				refreshErr := func() error {
					rc, err := c.Cookie(authpkg.RefreshTokenCookieName)
					if err != nil {
						return unauthorizedError("Failed to generate access token. Missing refresh token.")
					}

					refreshTokenClaims := &Claims{}
					refreshToken, err := jwt.ParseWithClaims(rc.Value, refreshTokenClaims, func(t *jwt.Token) (any, error) {
						if t.Method.Alg() != jwt.SigningMethodHS256.Name {
							return nil, errors.Errorf("unexpected refresh token signing method=%v, expected %v", t.Header["alg"], jwt.SigningMethodHS256)
						}
						if kid, ok := t.Header["kid"].(string); ok && kid == "v1" {
							return []byte(secret), nil
						}
						return nil, errors.Errorf("unexpected refresh token kid=%v", t.Header["kid"])
					})
					if err != nil {
						if errors.Is(err, jwt.ErrSignatureInvalid) {
							return unauthorizedError("Failed to generate access token. Invalid refresh token signature.")
						}
						return internalError(fmt.Sprintf("Server error to refresh expired token. User Id %d", userID), err)
					}
					if !audienceContains(refreshTokenClaims.Audience, authpkg.RefreshTokenAudienceName) {
						return unauthorizedError(fmt.Sprintf(
							"Invalid refresh token, audience mismatch, got %q, expected %q. you may send request to the wrong environment",
							refreshTokenClaims.Audience, authpkg.RefreshTokenAudienceName,
						))
					}
					if refreshToken != nil && refreshToken.Valid {
						if err := GenerateTokensAndSetCookies(c, user, secret); err != nil {
							return internalError(fmt.Sprintf("Server error to refresh expired token. User Id %d", userID), err)
						}
					}
					return nil
				}()
				if refreshErr != nil && (parsedAccessToken == nil || !parsedAccessToken.Valid) {
					return refreshErr
				}
			}

			c.Set(getUserIDContextKey(), userID)
			return next(c)
		}
	}
}

func audienceContains(audience jwt.ClaimStrings, token string) bool {
	for _, v := range audience {
		if v == token {
			return true
		}
	}
	return false
}
