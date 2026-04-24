package server

import (
	"encoding/json"
	"net/http"

	"github.com/usememos/memos/plugin/openai"
)

func (s *Server) registerOpenAIRoutes(g Group) {
	g.POST("/openai/chat-completion", func(c Context) error {
		ctx := c.Request().Context()

		messages := []openai.ChatCompletionMessage{}
		if err := json.NewDecoder(c.Request().Body).Decode(&messages); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post chat completion request", err)
		}
		if len(messages) == 0 {
			return newHTTPError(http.StatusBadRequest, "No messages provided")
		}

		result, err := s.Service.ChatCompletion(ctx, messages)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(result))
	})

	g.GET("/openai/enabled", func(c Context) error {
		ctx := c.Request().Context()
		cfg, err := s.Service.GetOpenAIConfig(ctx)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(cfg.Key != ""))
	})
}
