package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostChatCompletion(t *testing.T) {
	server := newOpenAITestServer(t, func(t *testing.T, body map[string]any) any {
		require.Equal(t, "gpt-3.5-turbo", body["model"])
		messages, ok := body["messages"].([]any)
		require.True(t, ok)
		require.Len(t, messages, 1)
		message, ok := messages[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "user", message["role"])
		require.Equal(t, "hello", message["content"])
		return ChatCompletionResponse{
			Choices: []ChatCompletionChoice{
				{Message: &ChatCompletionMessage{Role: "assistant", Content: "world"}},
			},
		}
	})
	defer server.Close()

	completion, err := PostChatCompletion([]ChatCompletionMessage{{Role: "user", Content: "hello"}}, "test-key", server.URL)
	require.NoError(t, err)
	require.Equal(t, "world", completion)
}

func TestPostTextCompletion(t *testing.T) {
	server := newOpenAITestServer(t, func(t *testing.T, body map[string]any) any {
		require.Equal(t, "gpt-3.5-turbo", body["model"])
		require.Equal(t, "summarize this", body["prompt"])
		require.Equal(t, float64(100), body["max_tokens"])
		return TextCompletionResponse{
			Choices: []TextCompletionChoice{{Text: "summary"}},
		}
	})
	defer server.Close()

	completion, err := PostTextCompletion("summarize this", "test-key", server.URL)
	require.NoError(t, err)
	require.Equal(t, "summary", completion)
}

func TestOpenAICompletionErrorResponse(t *testing.T) {
	server := newOpenAITestServer(t, func(t *testing.T, body map[string]any) any {
		return map[string]any{"error": map[string]any{"message": "quota exceeded"}}
	})
	defer server.Close()

	_, err := PostChatCompletion([]ChatCompletionMessage{{Role: "user", Content: "hello"}}, "test-key", server.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quota exceeded")
}

func TestOpenAICompletionEmptyChoices(t *testing.T) {
	server := newOpenAITestServer(t, func(t *testing.T, body map[string]any) any {
		return ChatCompletionResponse{}
	})
	defer server.Close()

	completion, err := PostChatCompletion([]ChatCompletionMessage{{Role: "user", Content: "hello"}}, "test-key", server.URL)
	require.NoError(t, err)
	require.Empty(t, completion)
}

func TestOpenAICompletionInvalidHost(t *testing.T) {
	_, err := PostChatCompletion(nil, "test-key", "://bad-host")
	require.Error(t, err)
}

func newOpenAITestServer(t *testing.T, response func(*testing.T, map[string]any) any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response(t, body)))
	}))
}

func TestOpenAICompletionInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := strings.NewReader("{invalid-json").WriteTo(w)
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := PostTextCompletion("prompt", "test-key", server.URL)
	require.Error(t, err)
}
