package server

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/usememos/memos/common"
)

const ginSessionStoreContextKey = "session-store"

func newGinApp() App {
	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(gin.Recovery())
	return &ginApp{
		app: app,
		server: &http.Server{
			Handler: app,
		},
	}
}

type ginApp struct {
	app    *gin.Engine
	server *http.Server
}

func (a *ginApp) Group(prefix string) Group {
	return &ginGroup{group: a.app.Group(prefix)}
}

func (a *ginApp) UseLogger(_ string) {
	a.app.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("{\"time\":\"%s\",\"method\":\"%s\",\"uri\":\"%s\",\"status\":%d,\"error\":\"%s\"}\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.ErrorMessage,
		)
	}))
}

func (a *ginApp) UseGzip() {
	a.app.Use(func(c *gin.Context) {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") || strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Next()
			return
		}

		writer := newGinGzipWriter(c.Writer)
		defer func() {
			if err := writer.Close(); err != nil {
				_ = c.Error(err)
			}
		}()

		appendVaryHeader(c.Writer.Header(), "Accept-Encoding")
		c.Header("Content-Encoding", "gzip")
		c.Header("Content-Length", "")
		c.Writer = writer
		c.Next()
	})
}

func (a *ginApp) UseCSRF(tokenLookup string, skipper func(Context) bool) {
	cookieName := csrfCookieName(tokenLookup)
	a.app.Use(func(c *gin.Context) {
		ctx := newGinContext(c)
		if skipper != nil && skipper(ctx) {
			c.Next()
			return
		}

		token, err := ensureCSRFCookie(c, cookieName)
		if err != nil {
			writeGinError(c, internalError("Failed to prepare CSRF cookie", err))
			c.Abort()
			return
		}

		if !isSafeHTTPMethod(c.Request.Method) {
			requestToken, err := c.Cookie(cookieName)
			if err != nil || requestToken == "" || requestToken != token {
				writeGinError(c, forbiddenError("Invalid CSRF token"))
				c.Abort()
				return
			}
		}

		c.Next()
	})
}

func (a *ginApp) UseCORS() {
	a.app.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	})
}

func (a *ginApp) UseSecure(config SecureConfig) {
	a.app.Use(func(c *gin.Context) {
		ctx := newGinContext(c)
		if config.Skipper != nil && config.Skipper(ctx) {
			c.Next()
			return
		}
		if config.XSSProtection != "" {
			c.Header("X-XSS-Protection", config.XSSProtection)
		}
		if config.ContentTypeNosniff != "" {
			c.Header("X-Content-Type-Options", config.ContentTypeNosniff)
		}
		if config.XFrameOptions != "" {
			c.Header("X-Frame-Options", config.XFrameOptions)
		}
		if config.HSTSPreloadEnabled {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		c.Next()
	})
}

func (a *ginApp) UseTimeout(timeout time.Duration, errorMessage string) {
	a.server.Handler = http.TimeoutHandler(a.server.Handler, timeout, errorMessage)
}

func (a *ginApp) UseSession(secret string) {
	store := sessions.NewCookieStore([]byte(secret))
	a.app.Use(func(c *gin.Context) {
		c.Set(ginSessionStoreContextKey, store)
		c.Next()
	})
}

func (a *ginApp) UseStatic(config StaticFileServerConfig) {
	if config.PathPrefix == "" {
		a.app.NoRoute(func(c *gin.Context) {
			ctx := newGinContext(c)
			if config.Skipper != nil && config.Skipper(ctx) {
				c.Status(http.StatusNotFound)
				return
			}
			serveStaticFile(c, config, c.Request.URL.Path)
		})
		return
	}

	group := a.app.Group(config.PathPrefix)
	wrappedGroup := &ginGroup{group: group}
	if len(config.Middlewares) > 0 {
		wrappedGroup.Use(config.Middlewares...)
	}
	handler := func(c *gin.Context) {
		ctx := newGinContext(c)
		if config.Skipper != nil && config.Skipper(ctx) {
			c.Status(http.StatusNotFound)
			return
		}
		serveStaticFile(c, config, strings.TrimPrefix(c.Request.URL.Path, config.PathPrefix))
	}
	group.GET("/*filepath", handler)
	group.HEAD("/*filepath", handler)
}

func (a *ginApp) Start(address string) error {
	a.server.Addr = address
	return a.server.ListenAndServe()
}

func (a *ginApp) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

type ginGroup struct {
	group *gin.RouterGroup
}

func (g *ginGroup) GET(path string, handler HandlerFunc) {
	g.group.GET(path, wrapGinHandler(handler))
}

func (g *ginGroup) POST(path string, handler HandlerFunc) {
	g.group.POST(path, wrapGinHandler(handler))
}

func (g *ginGroup) PATCH(path string, handler HandlerFunc) {
	g.group.PATCH(path, wrapGinHandler(handler))
}

func (g *ginGroup) DELETE(path string, handler HandlerFunc) {
	g.group.DELETE(path, wrapGinHandler(handler))
}

func (g *ginGroup) Use(middlewares ...MiddlewareFunc) {
	for _, middleware := range middlewares {
		g.group.Use(toGinMiddleware(middleware))
	}
}

func wrapGinHandler(handler HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := handler(newGinContext(c)); err != nil {
			writeGinError(c, err)
			c.Abort()
		}
	}
}

func toGinMiddleware(middleware MiddlewareFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := middleware(func(ctx Context) error {
			ginCtx := unwrapGinContext(ctx)
			ginCtx.Next()
			return nil
		})(newGinContext(c))
		if err != nil {
			writeGinError(c, err)
			c.Abort()
		}
	}
}

func writeGinError(c *gin.Context, err error) {
	var httpErr *httpError
	if unwrapHTTPError(err, &httpErr) {
		c.JSON(httpErr.code, gin.H{
			"error": httpErr.message,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": http.StatusText(http.StatusInternalServerError),
	})
}

type ginContext struct {
	context *gin.Context
}

func newGinContext(context *gin.Context) Context {
	return ginContext{context: context}
}

func unwrapGinContext(context Context) *gin.Context {
	ginContext, ok := context.(ginContext)
	if !ok {
		panic("server context adapter expected ginContext")
	}
	return ginContext.context
}

func (c ginContext) Request() *http.Request {
	return c.context.Request
}

func (c ginContext) Writer() http.ResponseWriter {
	return c.context.Writer
}

func (c ginContext) Cookie(name string) (*http.Cookie, error) {
	return c.context.Request.Cookie(name)
}

func (c ginContext) SetCookie(cookie *http.Cookie) {
	if !cookie.Secure && requestScheme(c.context.Request) == "https" {
		cookie.Secure = true
	}
	http.SetCookie(c.context.Writer, cookie)
}

func (c ginContext) JSON(code int, payload any) error {
	c.context.JSON(code, payload)
	return nil
}

func (c ginContext) String(code int, value string) error {
	c.context.String(code, value)
	return nil
}

func (c ginContext) Stream(code int, contentType string, reader io.Reader) error {
	c.context.Status(code)
	c.context.Header(headerContentType, contentType)
	_, err := io.Copy(c.context.Writer, reader)
	return err
}

func (c ginContext) Status(code int) {
	c.context.Status(code)
}

func (c ginContext) Header(key, value string) {
	c.context.Header(key, value)
}

func (c ginContext) Path() string {
	routePath := c.context.FullPath()
	if routePath == "" {
		return c.context.Request.URL.Path
	}
	return routePath
}

func (c ginContext) Param(name string) string {
	return c.context.Param(name)
}

func (c ginContext) QueryParam(name string) string {
	return c.context.Query(name)
}

func (c ginContext) Set(key string, value any) {
	c.context.Set(key, value)
}

func (c ginContext) Get(key string) any {
	value, ok := c.context.Get(key)
	if !ok {
		return nil
	}
	return value
}

func (c ginContext) FormFile(name string) (*multipart.FileHeader, error) {
	return c.context.FormFile(name)
}

func (c ginContext) Scheme() string {
	return requestScheme(c.context.Request)
}

func serveStaticFile(c *gin.Context, config StaticFileServerConfig, requestPath string) {
	filePath := strings.TrimPrefix(requestPath, "/")
	if filePath == "" {
		filePath = "index.html"
	}
	if _, err := config.Filesystem.Open(filePath); err != nil {
		if !config.HTML5 {
			c.Status(http.StatusNotFound)
			return
		}
		filePath = "index.html"
	}
	file, err := config.Filesystem.Open(filePath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	http.ServeContent(c.Writer, c.Request, filePath, fileInfo.ModTime(), file)
}

func requestScheme(request *http.Request) string {
	if request.TLS != nil {
		return "https"
	}
	if forwardedProto := request.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		return forwardedProto
	}
	if request.URL != nil && request.URL.Scheme != "" {
		return request.URL.Scheme
	}
	return "http"
}

type ginGzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func newGinGzipWriter(writer gin.ResponseWriter) *ginGzipWriter {
	return &ginGzipWriter{
		ResponseWriter: writer,
		writer:         gzip.NewWriter(writer),
	}
}

func (w *ginGzipWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *ginGzipWriter) WriteString(value string) (int, error) {
	return w.writer.Write([]byte(value))
}

func (w *ginGzipWriter) Flush() {
	_ = w.writer.Flush()
	w.ResponseWriter.Flush()
}

func (w *ginGzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *ginGzipWriter) Close() error {
	return w.writer.Close()
}

func appendVaryHeader(header http.Header, value string) {
	existing := header.Values("Vary")
	for _, entry := range existing {
		for _, part := range strings.Split(entry, ",") {
			if strings.TrimSpace(part) == value {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func csrfCookieName(tokenLookup string) string {
	const cookiePrefix = "cookie:"
	if strings.HasPrefix(tokenLookup, cookiePrefix) {
		cookieName := tokenLookup[len(cookiePrefix):]
		if cookieName != "" {
			return cookieName
		}
	}
	return "_csrf"
}

func ensureCSRFCookie(c *gin.Context, cookieName string) (string, error) {
	token, err := c.Cookie(cookieName)
	if err == nil && token != "" {
		return token, nil
	}

	token = common.GenUUID()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestScheme(c.Request) == "https",
		SameSite: http.SameSiteStrictMode,
	})
	return token, nil
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
