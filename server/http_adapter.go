package server

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	ninja "github.com/shijl0925/gin-ninja"
)

const (
	headerCacheControl          = "Cache-Control"
	headerContentSecurityPolicy = "Content-Security-Policy"
	headerContentType           = "Content-Type"
	headerXForwardedFor         = "X-Forwarded-For"
	headerXRealIP               = "X-Real-IP"
	mimeApplicationJSONCharset  = "application/json; charset=UTF-8"
	mimeApplicationXMLCharset   = "application/xml; charset=UTF-8"
	mimeTextPlain               = "text/plain; charset=UTF-8"
)

type Context interface {
	Request() *http.Request
	Writer() http.ResponseWriter
	Cookie(name string) (*http.Cookie, error)
	SetCookie(cookie *http.Cookie)
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
	AddController(prefix string, controller ninja.Controller, opts ...ninja.RouterOption)
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

func unauthorizedError(message string) error {
	return newHTTPError(http.StatusUnauthorized, message)
}

func forbiddenError(message string) error {
	return newHTTPError(http.StatusForbidden, message)
}

func internalError(message string, err error) error {
	return newHTTPErrorWithInternal(http.StatusInternalServerError, message, err)
}

func newApp() App {
	return newGinApp()
}

func unwrapHTTPError(err error, target **httpError) bool {
	httpErr, ok := err.(*httpError)
	if !ok {
		return false
	}
	*target = httpErr
	return true
}

func getClientIP(ctx Context) string {
	request := ctx.Request()
	if realIP := request.Header.Get(headerXRealIP); realIP != "" {
		return realIP
	}
	if forwardedFor := request.Header.Get(headerXForwardedFor); forwardedFor != "" {
		return forwardedFor
	}
	return request.RemoteAddr
}
