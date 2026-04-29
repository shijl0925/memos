package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/server/profile"
)

func TestGetSystemStatusSkipsDBFileStatForPostgres(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	host := createTestHostUser(ctx, svc, t)
	svc.Profile = &profile.Profile{
		Driver: "postgres",
		DSN:    "postgres://user:pass@example.com/memos",
	}

	status, err := svc.GetSystemStatus(ctx, &host.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), status.DBSize)
	require.Equal(t, api.Host, status.Host.Role)
}

func TestIsSQLiteDriver(t *testing.T) {
	require.True(t, isSQLiteDriver(""))
	require.True(t, isSQLiteDriver("sqlite"))
	require.True(t, isSQLiteDriver("sqlite3"))
	require.False(t, isSQLiteDriver("postgres"))
	require.False(t, isSQLiteDriver("mysql"))
}
