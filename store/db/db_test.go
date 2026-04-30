package db

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/server/version"
)

func TestSplitSQLStatements(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "empty script",
			script: " ; \n ; ",
			want:   nil,
		},
		{
			name:   "multiple statements",
			script: "CREATE TABLE memo(id INTEGER); INSERT INTO memo(id) VALUES(1);",
			want:   []string{"CREATE TABLE memo(id INTEGER)", "INSERT INTO memo(id) VALUES(1)"},
		},
		{
			name:   "ignores semicolons inside quoted strings",
			script: "INSERT INTO memo(content) VALUES('one;two'); SELECT \"semi;colon\"; SELECT `tag;name`",
			want: []string{
				"INSERT INTO memo(content) VALUES('one;two')",
				"SELECT \"semi;colon\"",
				"SELECT `tag;name`",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, splitSQLStatements(test.script))
		})
	}
}

func TestDBFormatQuery(t *testing.T) {
	query := "SELECT * FROM memo WHERE creator_id = ? AND visibility = ? LIMIT ?"
	require.Equal(t, "SELECT * FROM memo WHERE creator_id = $1 AND visibility = $2 LIMIT $3", dbFormatQuery("postgres", query))
	require.Equal(t, query, dbFormatQuery("sqlite3", query))
	require.Equal(t, query, dbFormatQuery("mysql", query))
}

func TestGetMinorVersionList(t *testing.T) {
	versions := getMinorVersionList()
	sortedVersions := append([]string(nil), versions...)
	sort.Sort(version.SortVersion(sortedVersions))
	require.Equal(t, sortedVersions, versions)

	for _, minorVersion := range versions {
		require.True(t, regexp.MustCompile(`^[0-9]+\.[0-9]+$`).MatchString(minorVersion))
	}
}

func TestMigrationHistorySQLite(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	_, err = sqlDB.ExecContext(ctx, `
		CREATE TABLE migration_history (
			version TEXT PRIMARY KEY,
			created_ts INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
		);
	`)
	require.NoError(t, err)

	testDB := &DB{
		DBInstance: sqlDB,
		profile:    &profile.Profile{Driver: "sqlite3"},
	}

	first, err := testDB.UpsertMigrationHistory(ctx, &MigrationHistoryUpsert{Version: "0.1.0"})
	require.NoError(t, err)
	require.Equal(t, "0.1.0", first.Version)
	require.NotZero(t, first.CreatedTs)

	second, err := testDB.UpsertMigrationHistory(ctx, &MigrationHistoryUpsert{Version: "0.2.0"})
	require.NoError(t, err)
	require.Equal(t, "0.2.0", second.Version)

	all, err := testDB.FindMigrationHistoryList(ctx, &MigrationHistoryFind{})
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, []string{"0.2.0", "0.1.0"}, []string{all[0].Version, all[1].Version})

	filtered, err := testDB.FindMigrationHistoryList(ctx, &MigrationHistoryFind{Version: &first.Version})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, first.Version, filtered[0].Version)
}
