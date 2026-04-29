package server

import ninja "github.com/shijl0925/gin-ninja"

type serverController struct {
	middlewares []MiddlewareFunc
	register    func(Group)
}

func (c serverController) Register(r *ninja.Router) {
	g := &ginNinjaGroup{router: r}
	if len(c.middlewares) > 0 {
		g.Use(c.middlewares...)
	}
	c.register(g)
}
