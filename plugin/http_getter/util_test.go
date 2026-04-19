package getter

import "testing"

func TestValidateOutboundURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "reject localhost",
			url:     "http://127.0.0.1:8080",
			wantErr: true,
		},
		{
			name:    "reject file scheme",
			url:     "file:///etc/passwd",
			wantErr: true,
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
	}
}
