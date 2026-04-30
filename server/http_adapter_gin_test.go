package server

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	ninja "github.com/shijl0925/gin-ninja"
	"github.com/stretchr/testify/require"
)

func newTestGinApp(t *testing.T) *ginApp {
	t.Helper()

	app, ok := newGinApp().(*ginApp)
	require.True(t, ok)
	return app
}

func TestNewAppUsesGinByDefault(t *testing.T) {
	_, ok := newApp().(*ginApp)
	require.True(t, ok)
}

func TestGinUseGzip(t *testing.T) {
	app := newTestGinApp(t)
	app.UseGzip()
	app.AddController("", serverController{
		registerAPI: func(r *ninja.Router) {
			ninja.Get(r, "/hello", adaptNinjaHandler(func(c Context) error {
				return c.String(http.StatusOK, "hello world")
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	app.app.ServeHTTP(recorder, request)

	require.Equal(t, "gzip", recorder.Header().Get("Content-Encoding"))
	reader, err := gzip.NewReader(strings.NewReader(recorder.Body.String()))
	require.NoError(t, err)
	defer reader.Close()

	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(body))
}

func TestGinUseCSRFSetsCookieAndProtectsUnsafeRequests(t *testing.T) {
	app := newTestGinApp(t)
	app.UseCSRF("cookie:_csrf", nil)
	app.AddController("", serverController{
		registerAPI: func(r *ninja.Router) {
			ninja.Get(r, "/", adaptNinjaHandler(func(c Context) error {
				return c.String(http.StatusOK, "ok")
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
			ninja.Post(r, "/submit", adaptNinjaHandler(func(c Context) error {
				return c.String(http.StatusOK, "submitted")
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
		},
	})

	getRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	getRecorder := httptest.NewRecorder()
	app.app.ServeHTTP(getRecorder, getRequest)

	require.Equal(t, http.StatusOK, getRecorder.Code)
	cookies := getRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	postRequest := httptest.NewRequest(http.MethodPost, "/submit", nil)
	postRecorder := httptest.NewRecorder()
	app.app.ServeHTTP(postRecorder, postRequest)
	require.Equal(t, http.StatusForbidden, postRecorder.Code)

	protectedRequest := httptest.NewRequest(http.MethodPost, "/submit", nil)
	protectedRequest.AddCookie(cookies[0])
	protectedRecorder := httptest.NewRecorder()
	app.app.ServeHTTP(protectedRecorder, protectedRequest)
	require.Equal(t, http.StatusOK, protectedRecorder.Code)
	require.Equal(t, "submitted", protectedRecorder.Body.String())
}

func TestGinUseTimeout(t *testing.T) {
	app := newTestGinApp(t)
	app.UseTimeout(10*time.Millisecond, "Request timeout")
	app.AddController("", serverController{
		registerAPI: func(r *ninja.Router) {
			ninja.Get(r, "/slow", adaptNinjaHandler(func(c Context) error {
				time.Sleep(50 * time.Millisecond)
				return c.String(http.StatusOK, "done")
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/slow", nil)
	recorder := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Request timeout")
}

func TestGinAddControllerRegistersNinjaController(t *testing.T) {
	app := newTestGinApp(t)
	app.AddController("/api", serverController{
		middlewares: []MiddlewareFunc{
			func(next HandlerFunc) HandlerFunc {
				return func(c Context) error {
					c.Set("controller-middleware", true)
					return next(c)
				}
			},
		},
		registerAPI: func(r *ninja.Router) {
			ninja.Get(r, "/ping", adaptNinjaHandler(func(c Context) error {
				enabled, _ := c.Get("controller-middleware").(bool)
				return c.JSON(http.StatusOK, composeResponse(enabled))
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
			ninja.Get(r, "/fail", adaptNinjaHandler(func(c Context) error {
				return newHTTPError(http.StatusBadRequest, "controller error")
			}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	recorder := httptest.NewRecorder()
	app.app.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"data":true}`, recorder.Body.String())

	errorRequest := httptest.NewRequest(http.MethodGet, "/api/fail", nil)
	errorRecorder := httptest.NewRecorder()
	app.app.ServeHTTP(errorRecorder, errorRequest)
	require.Equal(t, http.StatusBadRequest, errorRecorder.Code)
	require.JSONEq(t, `{"error":"controller error"}`, errorRecorder.Body.String())
}

func TestDetailedErrorLogEnvDisabledByDefault(t *testing.T) {
	t.Setenv(detailedErrorLogEnvVarName, "")

	require.False(t, isDetailedErrorLogEnabled())
}

func TestDetailedErrorLogEnvEnabled(t *testing.T) {
	t.Setenv(detailedErrorLogEnvVarName, "true")

	require.True(t, isDetailedErrorLogEnabled())
}

func TestGinWriteErrorOnlyExposesGinErrorWhenDetailedLogsEnabled(t *testing.T) {
	internalErr := errors.New("database connection refused")

	for _, test := range []struct {
		name          string
		envValue      string
		wantGinErrors int
	}{
		{
			name:          "disabled",
			envValue:      "false",
			wantGinErrors: 0,
		},
		{
			name:          "enabled",
			envValue:      "true",
			wantGinErrors: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(detailedErrorLogEnvVarName, test.envValue)
			app := newTestGinApp(t)
			app.app.Use(func(c *gin.Context) {
				c.Next()
				require.Len(t, c.Errors, test.wantGinErrors)
			})
			app.AddController("", serverController{
				registerAPI: func(r *ninja.Router) {
					ninja.Get(r, "/fail", adaptNinjaHandler(func(c Context) error {
						return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch memo list", internalErr)
					}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
				},
			})

			request := httptest.NewRequest(http.MethodGet, "/fail?limit=20", nil)
			request.Header.Set(headerXRealIP, "192.0.2.10")
			recorder := httptest.NewRecorder()
			app.app.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusInternalServerError, recorder.Code)
			require.JSONEq(t, `{"error":"Failed to fetch memo list"}`, recorder.Body.String())
		})
	}
}

func TestGinAddControllerRegistersVoidNinjaHandler(t *testing.T) {
	app := newTestGinApp(t)
	app.AddController("/api", serverController{
		registerAPI: func(r *ninja.Router) {
			ninja.Delete(r, "/memo/:id", adaptNinjaVoidHandler(func(c Context) error {
				c.Status(http.StatusNoContent)
				return nil
			}), ninja.ExcludeFromDocs())
		},
	})

	request := httptest.NewRequest(http.MethodDelete, "/api/memo/1", nil)
	recorder := httptest.NewRecorder()
	app.app.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Empty(t, recorder.Body.String())
}
