package server

import (
	"net/http"

	ninja "github.com/shijl0925/gin-ninja"
)

type ginNinjaGroup struct {
	router *ninja.Router
}

type emptyNinjaInput struct{}
type emptyNinjaOutput struct{}

func (g *ginNinjaGroup) GET(path string, handler HandlerFunc) {
	ninja.Get(g.router, path, adaptNinjaHandler(handler), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}

func (g *ginNinjaGroup) POST(path string, handler HandlerFunc) {
	ninja.Post(g.router, path, adaptNinjaHandler(handler), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}

func (g *ginNinjaGroup) PATCH(path string, handler HandlerFunc) {
	ninja.Patch(g.router, path, adaptNinjaHandler(handler), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}

func (g *ginNinjaGroup) DELETE(path string, handler HandlerFunc) {
	ninja.Delete(g.router, path, adaptNinjaVoidHandler(handler), ninja.ExcludeFromDocs())
}

func (g *ginNinjaGroup) Use(middlewares ...MiddlewareFunc) {
	for _, middleware := range middlewares {
		g.router.UseGin(toGinMiddleware(middleware))
	}
}

func adaptNinjaHandler(handler HandlerFunc) func(*ninja.Context, *emptyNinjaInput) (*emptyNinjaOutput, error) {
	return func(c *ninja.Context, _ *emptyNinjaInput) (*emptyNinjaOutput, error) {
		if err := handler(newGinContext(c.Context)); err != nil {
			writeGinError(c.Context, err)
			c.Abort()
		}
		return nil, nil
	}
}

func adaptNinjaVoidHandler(handler HandlerFunc) func(*ninja.Context, *emptyNinjaInput) error {
	return func(c *ninja.Context, _ *emptyNinjaInput) error {
		if err := handler(newGinContext(c.Context)); err != nil {
			writeGinError(c.Context, err)
			c.Abort()
		}
		return nil
	}
}

