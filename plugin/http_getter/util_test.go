package getter

import (
	"strings"
	"testing"
)

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

	for _, test := range tests {
		_, err := validateOutboundURL(test.url)
		if (err != nil) != test.wantErr {
			t.Fatalf("%s: got err=%v, wantErr=%v", test.name, err, test.wantErr)
		}
		if test.errMessage != "" && (err == nil || !strings.Contains(err.Error(), test.errMessage)) {
			t.Fatalf("%s: got err=%v, want substring=%q", test.name, err, test.errMessage)
		}
	}
}
