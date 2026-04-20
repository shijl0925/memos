package server

import (
	"strings"
	"testing"

	"github.com/usememos/memos/api"
)

func TestValidateMemoContentLength(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "within limit",
			content: strings.Repeat("a", api.MaxContentLength),
			wantErr: false,
		},
		{
			name:    "overflow",
			content: strings.Repeat("a", api.MaxContentLength+1),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMemoContentLength(test.content)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				httpErr, ok := err.(*httpError)
				if !ok {
					t.Fatalf("unexpected error type %T", err)
				}
				if httpErr.code != 400 {
					t.Fatalf("status code = %d, want 400", httpErr.code)
				}
				if httpErr.message != memoContentLengthOverflowMessage {
					t.Fatalf("message = %q, want %q", httpErr.message, memoContentLengthOverflowMessage)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
