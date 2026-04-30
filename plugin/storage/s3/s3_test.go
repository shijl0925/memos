package s3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "bucket",
		EndPoint:  "https://s3.example.com",
		Region:    "us-east-1",
		URLPrefix: "https://cdn.example.com",
		URLSuffix: "?v=1",
	}

	client, err := NewClient(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, client.Client)
	require.Equal(t, config, client.Config)
}
