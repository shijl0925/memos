package server

import (
	"encoding/json"
	"net/http"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/openai"
)

func (s *Server) registerOpenAIRoutes(g Group) {
	g.POST("/openai/chat-completion", func(c Context) error {
		ctx := c.Request().Context()
		openAIConfigSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingOpenAIConfigName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find openai key", err)
		}

		openAIConfig := api.OpenAIConfig{}
		if openAIConfigSetting != nil {
			if err := json.Unmarshal([]byte(openAIConfigSetting.Value), &openAIConfig); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal openai system setting value", err)
			}
		}
		if openAIConfig.Key == "" {
			return newHTTPError(http.StatusBadRequest, "OpenAI API key not set")
		}

		completionRequest := api.OpenAICompletionRequest{}
		if err := json.NewDecoder(c.Request().Body).Decode(&completionRequest); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post chat completion request", err)
		}
		if completionRequest.Prompt == "" {
			return newHTTPError(http.StatusBadRequest, "Prompt is required")
		}

		result, err := openai.PostChatCompletion(completionRequest.Prompt, openAIConfig.Key, openAIConfig.Host)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to post chat completion", err)
		}

		return c.JSON(http.StatusOK, composeResponse(result))
	})

	g.POST("/openai/text-completion", func(c Context) error {
		ctx := c.Request().Context()
		openAIConfigSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingOpenAIConfigName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find openai key", err)
		}

		openAIConfig := api.OpenAIConfig{}
		if openAIConfigSetting != nil {
			if err := json.Unmarshal([]byte(openAIConfigSetting.Value), &openAIConfig); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal openai system setting value", err)
			}
		}
		if openAIConfig.Key == "" {
			return newHTTPError(http.StatusBadRequest, "OpenAI API key not set")
		}

		textCompletion := api.OpenAICompletionRequest{}
		if err := json.NewDecoder(c.Request().Body).Decode(&textCompletion); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post text completion request", err)
		}
		if textCompletion.Prompt == "" {
			return newHTTPError(http.StatusBadRequest, "Prompt is required")
		}

		result, err := openai.PostTextCompletion(textCompletion.Prompt, openAIConfig.Key, openAIConfig.Host)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to post text completion", err)
		}

		return c.JSON(http.StatusOK, composeResponse(result))
	})

	g.GET("/openai/enabled", func(c Context) error {
		ctx := c.Request().Context()
		openAIConfigSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingOpenAIConfigName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find openai key", err)
		}

		openAIConfig := api.OpenAIConfig{}
		if openAIConfigSetting != nil {
			if err := json.Unmarshal([]byte(openAIConfigSetting.Value), &openAIConfig); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal openai system setting value", err)
			}
		}
		if openAIConfig.Key == "" {
			return newHTTPError(http.StatusBadRequest, "OpenAI API key not set")
		}

		return c.JSON(http.StatusOK, composeResponse(openAIConfig.Key != ""))
	})
}
