package server

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	headerCacheControl          = "Cache-Control"
	headerContentSecurityPolicy = "Content-Security-Policy"
	headerContentType           = "Content-Type"
	mimeApplicationJSONCharset  = "application/json; charset=UTF-8"
	mimeApplicationXMLCharset   = "application/xml; charset=UTF-8"
	mimeTextPlain               = "text/plain; charset=UTF-8"
)

type Context interface {
	Request() *http.Request
	Writer() http.ResponseWriter
	JSON(code int, payload any) error
	String(code int, value string) error
	Stream(code int, contentType string, reader io.Reader) error
	Status(code int)
	Header(key, value string)
	Path() string
	Param(name string) string
	QueryParam(name string) string
	Set(key string, value any)
	Get(key string) any
	FormFile(name string) (*multipart.FileHeader, error)
	Scheme() string
}

type HandlerFunc func(Context) error

type MiddlewareFunc func(HandlerFunc) HandlerFunc

type App interface {
	Group(prefix string) Group
	UseLogger(format string)
	UseGzip()
	UseCSRF(tokenLookup string, skipper func(Context) bool)
	UseCORS()
	UseSecure(config SecureConfig)
	UseTimeout(timeout time.Duration, errorMessage string)
	UseSession(secret string)
	UseStatic(config StaticFileServerConfig)
	Start(address string) error
	Shutdown(ctx context.Context) error
}

type Group interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
	PATCH(path string, handler HandlerFunc)
	DELETE(path string, handler HandlerFunc)
	Use(middlewares ...MiddlewareFunc)
}

type SecureConfig struct {
	Skipper            func(Context) bool
	XSSProtection      string
	ContentTypeNosniff string
	XFrameOptions      string
	HSTSPreloadEnabled bool
}

type StaticFileServerConfig struct {
	PathPrefix  string
	HTML5       bool
	Filesystem  http.FileSystem
	Skipper     func(Context) bool
	Middlewares []MiddlewareFunc
}

type httpError struct {
	code     int
	message  string
	internal error
}

func (e *httpError) Error() string {
	return e.message
}

func newHTTPError(code int, message string) error {
	return &httpError{code: code, message: message}
}

func newHTTPErrorWithInternal(code int, message string, err error) error {
	return &httpError{code: code, message: message, internal: err}
}

func badRequestError(message string, err error) error {
	if err != nil {
		return newHTTPErrorWithInternal(http.StatusBadRequest, message, err)
	}
	return newHTTPError(http.StatusBadRequest, message)
}

func unauthorizedError(message string) error {
	return newHTTPError(http.StatusUnauthorized, message)
}

func forbiddenError(message string) error {
	return newHTTPError(http.StatusForbidden, message)
}

func notFoundError(message string, err error) error {
	if err != nil {
		return newHTTPErrorWithInternal(http.StatusNotFound, message, err)
	}
	return newHTTPError(http.StatusNotFound, message)
}

func internalError(message string, err error) error {
	return newHTTPErrorWithInternal(http.StatusInternalServerError, message, err)
}

func writeJSON(c Context, data any) error {
	return c.JSON(http.StatusOK, composeResponse(data))
}

func echoSkipper(skipper func(Context) bool) func(echo.Context) bool {
	return func(c echo.Context) bool {
		if skipper == nil {
			return false
		}
		return skipper(newEchoContext(c))
	}
}

func newEchoApp() App {
	app := echo.New()
	app.Debug = true
	app.HideBanner = true
	app.HidePort = true
	return echoApp{app: app}
}

func toEchoMiddleware(middleware MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return middleware(func(ctx Context) error {
				return next(unwrapEchoContext(ctx))
			})(newEchoContext(c))
		}
	}
}

type echoApp struct {
	app *echo.Echo
}

func (a echoApp) Group(prefix string) Group {
	return newEchoGroup(a.app.Group(prefix))
}

func (a echoApp) UseLogger(format string) {
	a.app.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: format,
	}))
}

func (a echoApp) UseGzip() {
	a.app.Use(middleware.Gzip())
}

func (a echoApp) UseCSRF(tokenLookup string, skipper func(Context) bool) {
	a.app.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		Skipper:     echoSkipper(skipper),
		TokenLookup: tokenLookup,
	}))
}

func (a echoApp) UseCORS() {
	a.app.Use(middleware.CORS())
}

func (a echoApp) UseSecure(config SecureConfig) {
	a.app.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		Skipper:            echoSkipper(config.Skipper),
		XSSProtection:      config.XSSProtection,
		ContentTypeNosniff: config.ContentTypeNosniff,
		XFrameOptions:      config.XFrameOptions,
		HSTSPreloadEnabled: config.HSTSPreloadEnabled,
	}))
}

func (a echoApp) UseTimeout(timeout time.Duration, errorMessage string) {
	a.app.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		ErrorMessage: errorMessage,
		Timeout:      timeout,
	}))
}

func (a echoApp) UseSession(secret string) {
	a.app.Use(session.Middleware(sessions.NewCookieStore([]byte(secret))))
}

func (a echoApp) UseStatic(config StaticFileServerConfig) {
	staticMiddleware := middleware.StaticWithConfig(middleware.StaticConfig{
		Skipper:    echoSkipper(config.Skipper),
		HTML5:      config.HTML5,
		Filesystem: config.Filesystem,
	})

	if config.PathPrefix == "" {
		a.app.Use(staticMiddleware)
		return
	}

	group := a.app.Group(config.PathPrefix)
	for _, mw := range config.Middlewares {
		group.Use(toEchoMiddleware(mw))
	}
	group.Use(staticMiddleware)
}

func (a echoApp) Start(address string) error {
	return a.app.Start(address)
}

func (a echoApp) Shutdown(ctx context.Context) error {
	return a.app.Shutdown(ctx)
}

func newEchoGroup(group *echo.Group) Group {
	return echoGroup{group: group}
}

type echoGroup struct {
	group *echo.Group
}

func (g echoGroup) GET(path string, handler HandlerFunc) {
	g.group.GET(path, wrapEchoHandler(handler))
}

func (g echoGroup) POST(path string, handler HandlerFunc) {
	g.group.POST(path, wrapEchoHandler(handler))
}

func (g echoGroup) PATCH(path string, handler HandlerFunc) {
	g.group.PATCH(path, wrapEchoHandler(handler))
}

func (g echoGroup) DELETE(path string, handler HandlerFunc) {
	g.group.DELETE(path, wrapEchoHandler(handler))
}

func (g echoGroup) Use(middlewares ...MiddlewareFunc) {
	for _, middleware := range middlewares {
		g.group.Use(toEchoMiddleware(middleware))
	}
}

func wrapEchoHandler(handler HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := handler(newEchoContext(c))
		if err == nil {
			return nil
		}

		var httpErr *httpError
		if ok := unwrapHTTPError(err, &httpErr); ok {
			echoErr := echo.NewHTTPError(httpErr.code, httpErr.message)
			if httpErr.internal != nil {
				echoErr.SetInternal(httpErr.internal)
			}
			return echoErr
		}
		return err
	}
}

func unwrapHTTPError(err error, target **httpError) bool {
	httpErr, ok := err.(*httpError)
	if !ok {
		return false
	}
	*target = httpErr
	return true
}

type echoContext struct {
	context echo.Context
}

func newEchoContext(context echo.Context) Context {
	return echoContext{context: context}
}

func unwrapEchoContext(context Context) echo.Context {
	echoContext, ok := context.(echoContext)
	if !ok {
		panic("server context adapter expected echoContext")
	}
	return echoContext.context
}

func (c echoContext) Request() *http.Request {
	return c.context.Request()
}

func (c echoContext) Writer() http.ResponseWriter {
	return c.context.Response().Writer
}

func (c echoContext) JSON(code int, payload any) error {
	return c.context.JSON(code, payload)
}

func (c echoContext) String(code int, value string) error {
	return c.context.String(code, value)
}

func (c echoContext) Stream(code int, contentType string, reader io.Reader) error {
	return c.context.Stream(code, contentType, reader)
}

func (c echoContext) Status(code int) {
	c.context.Response().WriteHeader(code)
}

func (c echoContext) Header(key, value string) {
	c.context.Response().Header().Set(key, value)
}

func (c echoContext) Path() string {
	return c.context.Path()
}

func (c echoContext) Param(name string) string {
	return c.context.Param(name)
}

func (c echoContext) QueryParam(name string) string {
	return c.context.QueryParam(name)
}

func (c echoContext) Set(key string, value any) {
	c.context.Set(key, value)
}

func (c echoContext) Get(key string) any {
	return c.context.Get(key)
}

func (c echoContext) FormFile(name string) (*multipart.FileHeader, error) {
	return c.context.FormFile(name)
}

func (c echoContext) Scheme() string {
	return c.context.Scheme()
}

func getSession(name string, ctx Context) (*sessions.Session, error) {
	return session.Get(name, unwrapEchoContext(ctx))
}

func getClientIP(ctx Context) string {
	request := ctx.Request()
	if realIP := request.Header.Get(echo.HeaderXRealIP); realIP != "" {
		return realIP
	}
	if forwardedFor := request.Header.Get(echo.HeaderXForwardedFor); forwardedFor != "" {
		return forwardedFor
	}
	return request.RemoteAddr
}
