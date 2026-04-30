package server

import (
	"encoding/json"
	ninja "github.com/shijl0925/gin-ninja"
	"net/http"

	"github.com/usememos/memos/plugin/openai"
)

func (s *Server) registerOpenAIRoutes(r *ninja.Router) {
	ninja.Post(r, "/openai/chat-completion", adaptNinjaHandler(func(c Context) error {
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
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/openai/enabled", adaptNinjaJSONHandler(func(c *ninja.Context, _ *struct{}) (bool, error) {
		ctx := c.Request.Context()
		cfg, err := s.Service.GetOpenAIConfig(ctx)
		if err != nil {
			return false, convertServiceError(err)
		}
		return cfg.Key != "", nil
	}), ninja.SuccessStatus(http.StatusOK))
}
