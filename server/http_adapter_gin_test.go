package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	app.Group("").GET("/hello", func(c Context) error {
		return c.String(http.StatusOK, "hello world")
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

func TestGinUseTimeout(t *testing.T) {
	app := newTestGinApp(t)
	app.UseTimeout(10*time.Millisecond, "Request timeout")
	app.Group("").GET("/slow", func(c Context) error {
		time.Sleep(50 * time.Millisecond)
		return c.String(http.StatusOK, "done")
	})

	request := httptest.NewRequest(http.MethodGet, "/slow", nil)
	recorder := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Request timeout")
}
