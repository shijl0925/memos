package server

import (
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
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

type Group interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
	PATCH(path string, handler HandlerFunc)
	DELETE(path string, handler HandlerFunc)
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
		return skipper(newEchoContext(c))
	}
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

func wrapEchoHandler(handler HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := handler(newEchoContext(c))
		if err == nil {
			return nil
		}

		var httpErr *httpError
		if ok := AsHTTPError(err, &httpErr); ok {
			echoErr := echo.NewHTTPError(httpErr.code, httpErr.message)
			if httpErr.internal != nil {
				echoErr.SetInternal(httpErr.internal)
			}
			return echoErr
		}
		return err
	}
}

func AsHTTPError(err error, target **httpError) bool {
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
		panic("unsupported context adapter")
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
