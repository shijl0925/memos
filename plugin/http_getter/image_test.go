package getter

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImage(t *testing.T) {
	withTestHTTPClient(t, func(request *http.Request) (*http.Response, error) {
		return newTestResponse(request, "image/webp", "image-bytes"), nil
	})

	tests := []struct {
		urlStr string
	}{
		{
			urlStr: "https://example.com/bytebase.webp",
		},
	}
	for _, test := range tests {
		_, err := GetImage(test.urlStr)
		require.NoError(t, err)
	}
}
