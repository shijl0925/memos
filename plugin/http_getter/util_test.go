package getter

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func withTestHTTPClient(t *testing.T, fn roundTripFunc) {
	t.Helper()

	originalClient := defaultHTTPClient
	defaultHTTPClient = &http.Client{
		Transport: fn,
	}
	t.Cleanup(func() {
		defaultHTTPClient = originalClient
	})
}

func newTestResponse(request *http.Request, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: request,
	}
}

func TestValidateOutboundURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantErr    bool
		errMessage string
	}{
		{
			name:       "reject localhost",
			url:        "http://127.0.0.1:8080",
			wantErr:    true,
			errMessage: "outbound URL host is not public",
		},
		{
			name:       "reject file scheme",
			url:        "file:///etc/passwd",
			wantErr:    true,
			errMessage: "unsupported outbound URL scheme",
		},
		{
			name:    "allow public ip",
			url:     "https://1.1.1.1/image.png",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		_, err := validateOutboundURL(tc.url)
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: got err=%v, wantErr=%v", tc.name, err, tc.wantErr)
		}
		if tc.errMessage != "" && (err == nil || !strings.Contains(err.Error(), tc.errMessage)) {
			t.Fatalf("%s: got err=%v, want substring=%q", tc.name, err, tc.errMessage)
		}
	}
}
