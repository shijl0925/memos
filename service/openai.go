package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/openai"
)

// GetOpenAIConfig reads the OpenAI configuration from system settings.
func (s *Service) GetOpenAIConfig(ctx context.Context) (*api.OpenAIConfig, error) {
	setting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingOpenAIConfigName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("failed to find openai setting: %w", err)
	}
	cfg := &api.OpenAIConfig{}
	if setting != nil {
		if err := json.Unmarshal([]byte(setting.Value), cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal openai config: %w", err)
		}
	}
	return cfg, nil
}

// ChatCompletion validates that an API key is configured and forwards the
// messages to the OpenAI API.
func (s *Service) ChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	cfg, err := s.GetOpenAIConfig(ctx)
	if err != nil {
		return "", err
	}
	if cfg.Key == "" {
		return "", common.Errorf(common.Invalid, fmt.Errorf("OpenAI API key not set"))
	}
	result, err := openai.PostChatCompletion(messages, cfg.Key, cfg.Host)
	if err != nil {
		return "", fmt.Errorf("failed to post chat completion: %w", err)
	}
	return result, nil
}
