package server

import ninja "github.com/shijl0925/gin-ninja"

type emptyNinjaInput struct{}
type emptyNinjaOutput struct{}

type ninjaResponse[T any] struct {
	Data T `json:"data"`
}

func adaptNinjaJSONHandler[TIn any, TOut any](handler func(*ninja.Context, *TIn) (TOut, error)) func(*ninja.Context, *TIn) (*ninjaResponse[TOut], error) {
	return func(c *ninja.Context, in *TIn) (*ninjaResponse[TOut], error) {
		out, err := handler(c, in)
		if err != nil {
			writeGinError(c.Context, err)
			c.Abort()
			return nil, nil
		}
		return &ninjaResponse[TOut]{Data: out}, nil
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
