package server

import ninja "github.com/shijl0925/gin-ninja"

type serverController struct {
	middlewares []MiddlewareFunc
	registerAPI func(*ninja.Router)
}

func (c serverController) Register(r *ninja.Router) {
	for _, middleware := range c.middlewares {
		r.UseGin(toGinMiddleware(middleware))
	}
	if c.registerAPI != nil {
		c.registerAPI(r)
	}
}
