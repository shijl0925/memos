package server

import ninja "github.com/shijl0925/gin-ninja"

type emptyNinjaInput struct{}
type emptyNinjaOutput struct{}

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
