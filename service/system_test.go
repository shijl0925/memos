package service

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
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

func TestGetSystemStatusAppliesSettingsAndSQLiteDBSize(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	host := createTestHostUser(ctx, svc, t)

	dbFile, err := os.CreateTemp(t.TempDir(), "memos-*.db")
	require.NoError(t, err)
	_, err = dbFile.WriteString("database")
	require.NoError(t, err)
	require.NoError(t, dbFile.Close())
	svc.Profile = &profile.Profile{Driver: "sqlite3", DSN: dbFile.Name()}

	settings := []*api.SystemSettingUpsert{
		{Name: api.SystemSettingAllowSignUpName, Value: "true"},
		{Name: api.SystemSettingIgnoreUpgradeName, Value: "true"},
		{Name: api.SystemSettingDisablePublicMemosName, Value: "true"},
		{Name: api.SystemSettingAdditionalStyleName, Value: `"body { color: red; }"`},
		{Name: api.SystemSettingAdditionalScriptName, Value: `"console.log('ok')"`},
		{Name: api.SystemSettingCustomizedProfileName, Value: `{"name":"test","logoUrl":"logo","description":"desc","locale":"en","appearance":"dark","externalUrl":"https://example.com"}`},
		{Name: api.SystemSettingStorageServiceIDName, Value: "-1"},
		{Name: api.SystemSettingLocalStoragePathName, Value: `"/tmp/memos"`},
		{Name: api.SystemSettingServerIDName, Value: `"server-id"`},
		{Name: api.SystemSettingSecretSessionName, Value: `"secret"`},
		{Name: api.SystemSettingOpenAIConfigName, Value: `{"key":"secret"}`},
	}
	for _, setting := range settings {
		_, err := svc.Store.UpsertSystemSetting(ctx, setting)
		require.NoError(t, err)
	}

	status, err := svc.GetSystemStatus(ctx, &host.ID)
	require.NoError(t, err)
	require.True(t, status.AllowSignUp)
	require.True(t, status.IgnoreUpgrade)
	require.True(t, status.DisablePublicMemos)
	require.Equal(t, "body { color: red; }", status.AdditionalStyle)
	require.Equal(t, "console.log('ok')", status.AdditionalScript)
	require.Equal(t, "test", status.CustomizedProfile.Name)
	require.Equal(t, api.LocalStorage, status.StorageServiceID)
	require.Equal(t, "/tmp/memos", status.LocalStoragePath)
	require.Equal(t, int64(len("database")), status.DBSize)
	require.Empty(t, status.Host.Email)
	require.Empty(t, status.Host.OpenID)
}

func TestSystemSettingAuthorization(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	host := createTestHostUser(ctx, svc, t)
	normalUser := createTestUser(ctx, svc, t, "normal", api.NormalUser)

	_, err := svc.UpsertSystemSetting(ctx, normalUser.ID, &api.SystemSettingUpsert{
		Name:  api.SystemSettingAllowSignUpName,
		Value: "true",
	})
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))

	_, err = svc.UpsertSystemSetting(ctx, host.ID, &api.SystemSettingUpsert{
		Name:  api.SystemSettingAllowSignUpName,
		Value: "not-json-bool",
	})
	require.Error(t, err)
	require.Equal(t, common.Invalid, common.ErrorCode(err))

	setting, err := svc.UpsertSystemSetting(ctx, host.ID, &api.SystemSettingUpsert{
		Name:  api.SystemSettingAllowSignUpName,
		Value: "true",
	})
	require.NoError(t, err)
	require.Equal(t, api.SystemSettingAllowSignUpName, setting.Name)

	_, err = svc.GetSystemSettingList(ctx, normalUser.ID)
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))

	settings, err := svc.GetSystemSettingList(ctx, host.ID)
	require.NoError(t, err)
	require.Len(t, settings, 1)

	err = svc.VacuumDatabase(ctx, normalUser.ID)
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
	require.NoError(t, svc.VacuumDatabase(ctx, host.ID))
}

func TestSystemGeneratedSettingsAndCustomizedProfile(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	serverID, err := svc.GetSystemServerID(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, serverID)
	sameServerID, err := svc.GetSystemServerID(ctx)
	require.NoError(t, err)
	require.Equal(t, serverID, sameServerID)

	secret, err := svc.GetSystemSecretSession(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	sameSecret, err := svc.GetSystemSecretSession(ctx)
	require.NoError(t, err)
	require.Equal(t, secret, sameSecret)

	defaultProfile, err := svc.GetSystemCustomizedProfile(ctx)
	require.NoError(t, err)
	require.Equal(t, "memos", defaultProfile.Name)

	_, err = svc.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:  api.SystemSettingCustomizedProfileName,
		Value: `{"name":"custom","locale":"en","appearance":"dark"}`,
	})
	require.NoError(t, err)
	customProfile, err := svc.GetSystemCustomizedProfile(ctx)
	require.NoError(t, err)
	require.Equal(t, "custom", customProfile.Name)
	require.Equal(t, "dark", customProfile.Appearance)
}
