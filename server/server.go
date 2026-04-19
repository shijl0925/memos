package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/store"

	"github.com/gorilla/securecookie"
)

type Server struct {
	e *gin.Engine

	Profile *profile.Profile

	Store *store.Store
}

func NewServer(profile *profile.Profile) *Server {
	gin.SetMode(gin.DebugMode)
	if profile.Mode == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	e := gin.New()
	e.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s %d\n", param.Method, param.Path, param.StatusCode)
	}))
	e.Use(gin.Recovery())

	// In dev mode, set the const secret key to make login session persistence.
	secret := []byte("usememos")
	if profile.Mode == "prod" {
		secret = securecookie.GenerateRandomKey(16)
	}
	store := cookie.NewStore(secret)
	e.Use(sessions.Sessions("session", store))

	s := &Server{
		e:       e,
		Profile: profile,
	}

	// Webhooks api skips auth checker.
	webhookGroup := e.Group("/h")
	s.registerWebhookRoutes(webhookGroup)

	apiGroup := e.Group("/api")
	apiGroup.Use(BasicAuthMiddleware(s))
	s.registerSystemRoutes(apiGroup)
	s.registerAuthRoutes(apiGroup)
	s.registerUserRoutes(apiGroup)
	s.registerMemoRoutes(apiGroup)
	s.registerShortcutRoutes(apiGroup)
	s.registerResourceRoutes(apiGroup)
	s.registerTagRoutes(apiGroup)

	e.NoRoute(frontendHandler("web/dist"))

	return s
}

func (server *Server) Run() error {
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", server.Profile.Port),
		Handler:           http.TimeoutHandler(server.e, 30*time.Second, "Request timeout"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return httpServer.ListenAndServe()
}

func frontendHandler(distPath string) gin.HandlerFunc {
	distRoot, err := filepath.Abs(distPath)
	if err != nil {
		panic(fmt.Errorf("failed to resolve dist path: %w", err))
	}

	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if c.Request.URL.Path == "/" {
			c.File(filepath.Join(distRoot, "index.html"))
			return
		}

		filename := filepath.Clean(c.Request.URL.Path)
		filename = filename[1:]
		target := filepath.Join(distRoot, filename)
		target, err := filepath.Abs(target)
		if err == nil {
			rel, relErr := filepath.Rel(distRoot, target)
			if relErr == nil && rel != ".." && !strings.HasPrefix(rel, fmt.Sprintf("..%c", filepath.Separator)) {
				if info, statErr := os.Stat(target); statErr == nil && !info.IsDir() {
					c.File(target)
					return
				}
			}
		}

		c.File(filepath.Join(distRoot, "index.html"))
	}
}
