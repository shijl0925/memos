package server

import ninja "github.com/shijl0925/gin-ninja"

type serverController struct {
	middlewares []MiddlewareFunc
	register    func(Group)
	registerAPI func(*ninja.Router)
}

func (c serverController) Register(r *ninja.Router) {
	g := &ginNinjaGroup{router: r}
	if len(c.middlewares) > 0 {
		g.Use(c.middlewares...)
	}
	if c.registerAPI != nil {
		c.registerAPI(r)
	}
	if c.register != nil {
		c.register(g)
	}
}
