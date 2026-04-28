package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSystemStatusSkipsDBSizeForNonSQLiteHost(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	host := createTestHostUser(ctx, svc, t)
	svc.Profile.Driver = "postgres"
	svc.Profile.DSN = "postgres://memos.example.invalid/memos"

	status, err := svc.GetSystemStatus(ctx, &host.ID)
	require.NoError(t, err)
	require.NotNil(t, status.Host)
	require.Equal(t, int64(0), status.DBSize)
}
