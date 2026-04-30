package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestProfileIsDev(t *testing.T) {
	require.True(t, (&Profile{Mode: "dev"}).IsDev())
	require.True(t, (&Profile{Mode: "test"}).IsDev())
	require.False(t, (&Profile{Mode: "prod"}).IsDev())
}

func TestGetProfileDefaultsInvalidModeToDev(t *testing.T) {
	viper.Reset()
	viper.SetDefault("mode", "invalid")
	viper.SetDefault("port", 8081)
	viper.SetDefault("driver", "sqlite3")

	prof, err := GetProfile()
	require.NoError(t, err)
	require.Equal(t, "dev", prof.Mode)
	require.Equal(t, filepath.Join(prof.Data, "memos_dev.db"), prof.DSN)
}

func TestResolveDataDirReturnsErrorForMissingDirectory(t *testing.T) {
	_, err := resolveDataDir("dev", filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

func TestResolveDataDirCleansAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveDataDir("dev", filepath.Join(dir, "."))
	require.NoError(t, err)
	require.Equal(t, filepath.Clean(dir), got)
}

func TestResolveDataDirRelativeExistingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(cwd))
	})

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "data"), 0755))
	require.NoError(t, os.Chdir(root))

	got, err := resolveDataDir("dev", "data")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "data"), got)
}
