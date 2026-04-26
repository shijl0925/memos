package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/server/version"
)

//go:embed migration
var migrationFS embed.FS

//go:embed seed
var seedFS embed.FS

type DB struct {
	// db connection instance
	DBInstance *sql.DB
	profile    *profile.Profile
}

// NewDB returns a new instance of DB associated with the given datasource name.
func NewDB(profile *profile.Profile) *DB {
	db := &DB{
		profile: profile,
	}
	return db
}

func (db *DB) Open(ctx context.Context) (err error) {
	if db.profile.DSN == "" {
		return fmt.Errorf("dsn required")
	}

	switch db.profile.Driver {
	case "mysql":
		return db.openNonSQLite(ctx, "mysql")
	case "postgres":
		return db.openNonSQLite(ctx, "postgres")
	default:
		return db.openSQLite(ctx)
	}
}

func (db *DB) openSQLite(ctx context.Context) error {
	// Demo mode must always start with fresh seed data.  Any stale DB file
	// (e.g. left from a previous demo run) would cause the HOST user to be
	// missing or corrupted, which manifests as:
	//   - /auth showing only the Sign-Up button (no Sign-In)
	//   - sign-up as "demohero" returning 409 Conflict
	// Deleting the files before sql.Open ensures that the lazy-creation of
	// the SQLite file triggers a full schema + seed bootstrap below.
	if db.profile.Mode == "demo" {
		_ = os.Remove(db.profile.DSN)
		_ = os.Remove(db.profile.DSN + "-wal")
		_ = os.Remove(db.profile.DSN + "-shm")
	}

	sqliteDB, err := sql.Open("sqlite3", db.profile.DSN+"?cache=shared&_foreign_keys=0&_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to open db with dsn: %s, err: %w", db.profile.DSN, err)
	}
	db.DBInstance = sqliteDB

	if db.profile.Mode == "prod" {
		if _, err := os.Stat(db.profile.DSN); errors.Is(err, os.ErrNotExist) {
			if err := db.applyLatestSchema(ctx); err != nil {
				return fmt.Errorf("failed to apply latest schema: %w", err)
			}
		}

		currentVersion := version.GetCurrentVersion(db.profile.Mode)
		migrationHistoryList, err := db.FindMigrationHistoryList(ctx, &MigrationHistoryFind{})
		if err != nil {
			return fmt.Errorf("failed to find migration history, err: %w", err)
		}
		if len(migrationHistoryList) == 0 {
			_, err := db.UpsertMigrationHistory(ctx, &MigrationHistoryUpsert{
				Version: currentVersion,
			})
			if err != nil {
				return fmt.Errorf("failed to upsert migration history, err: %w", err)
			}
			return nil
		}

		migrationHistoryVersionList := []string{}
		for _, migrationHistory := range migrationHistoryList {
			migrationHistoryVersionList = append(migrationHistoryVersionList, migrationHistory.Version)
		}
		sort.Sort(version.SortVersion(migrationHistoryVersionList))
		latestMigrationHistoryVersion := migrationHistoryVersionList[len(migrationHistoryVersionList)-1]

		if version.IsVersionGreaterThan(version.GetSchemaVersion(currentVersion), latestMigrationHistoryVersion) {
			minorVersionList := getMinorVersionList()

			rawBytes, err := os.ReadFile(db.profile.DSN)
			if err != nil {
				return fmt.Errorf("failed to read raw database file, err: %w", err)
			}
			backupDBFilePath := fmt.Sprintf("%s/memos_%s_%d_backup.db", db.profile.Data, db.profile.Version, time.Now().Unix())
			if err := os.WriteFile(backupDBFilePath, rawBytes, 0644); err != nil {
				return fmt.Errorf("failed to write raw database file, err: %w", err)
			}
			println("succeed to copy a backup database file")

			println("start migrate")
			for _, minorVersion := range minorVersionList {
				normalizedVersion := minorVersion + ".0"
				if version.IsVersionGreaterThan(normalizedVersion, latestMigrationHistoryVersion) && version.IsVersionGreaterOrEqualThan(currentVersion, normalizedVersion) {
					println("applying migration for", normalizedVersion)
					if err := db.applyMigrationForMinorVersion(ctx, minorVersion); err != nil {
						return fmt.Errorf("failed to apply minor version migration: %w", err)
					}
				}
			}
			println("end migrate")

			if err := os.Remove(backupDBFilePath); err != nil {
				println(fmt.Sprintf("Failed to remove temp database file, err %v", err))
			}
		}
	} else {
		if _, err := os.Stat(db.profile.DSN); errors.Is(err, os.ErrNotExist) {
			if err := db.applyLatestSchema(ctx); err != nil {
				return fmt.Errorf("failed to apply latest schema: %w", err)
			}
			if db.profile.Mode == "demo" {
				if err := db.seed(ctx); err != nil {
					return fmt.Errorf("failed to seed: %w", err)
				}
			}
		}
	}

	return nil
}

func (db *DB) openNonSQLite(ctx context.Context, driverName string) error {
	sqlDB, err := sql.Open(driverName, db.profile.DSN)
	if err != nil {
		return fmt.Errorf("failed to open %s db with dsn: %s, err: %w", driverName, db.profile.DSN, err)
	}
	db.DBInstance = sqlDB

	exists, err := db.tableExists(ctx, "migration_history")
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		if err := db.applyLatestSchema(ctx); err != nil {
			return fmt.Errorf("failed to apply latest schema: %w", err)
		}
		currentVersion := version.GetCurrentVersion(db.profile.Mode)
		if _, err := db.UpsertMigrationHistory(ctx, &MigrationHistoryUpsert{Version: currentVersion}); err != nil {
			return fmt.Errorf("failed to upsert migration history, err: %w", err)
		}
	}

	return nil
}

func (db *DB) tableExists(ctx context.Context, tableName string) (bool, error) {
	var count int
	var err error
	switch db.profile.Driver {
	case "mysql":
		err = db.DBInstance.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, tableName).Scan(&count)
	case "postgres":
		err = db.DBInstance.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name=$1`, tableName).Scan(&count)
	default:
		err = db.DBInstance.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

const (
	latestSchemaFileName = "LATEST__SCHEMA.sql"
)

func (db *DB) applyLatestSchema(ctx context.Context) error {
	var schemaDir string
	switch db.profile.Driver {
	case "mysql":
		schemaDir = "migration/mysql"
	case "postgres":
		schemaDir = "migration/postgres"
	default:
		if db.profile.Mode == "prod" {
			schemaDir = "migration/prod"
		} else {
			schemaDir = "migration/dev"
		}
	}

	latestSchemaPath := fmt.Sprintf("%s/%s", schemaDir, latestSchemaFileName)
	buf, err := migrationFS.ReadFile(latestSchemaPath)
	if err != nil {
		return fmt.Errorf("failed to read latest schema %q, error %w", latestSchemaPath, err)
	}
	stmt := string(buf)
	if err := db.execute(ctx, stmt); err != nil {
		return fmt.Errorf("migrate error: statement:%s err=%w", stmt, err)
	}
	return nil
}

func (db *DB) applyMigrationForMinorVersion(ctx context.Context, minorVersion string) error {
	filenames, err := fs.Glob(migrationFS, fmt.Sprintf("%s/%s/*.sql", "migration/prod", minorVersion))
	if err != nil {
		return fmt.Errorf("failed to read ddl files, err: %w", err)
	}

	sort.Strings(filenames)
	migrationStmt := ""

	for _, filename := range filenames {
		buf, err := migrationFS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read minor version migration file, filename=%s err=%w", filename, err)
		}
		stmt := string(buf)
		migrationStmt += stmt
		if err := db.execute(ctx, stmt); err != nil {
			return fmt.Errorf("migrate error: statement:%s err=%w", stmt, err)
		}
	}

	tx, err := db.DBInstance.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	v := minorVersion + ".0"
	if _, err = upsertMigrationHistory(ctx, tx, db.profile.Driver, &MigrationHistoryUpsert{
		Version: v,
	}); err != nil {
		return fmt.Errorf("failed to upsert migration history with version: %s, err: %w", v, err)
	}

	return tx.Commit()
}

func (db *DB) seed(ctx context.Context) error {
	filenames, err := fs.Glob(seedFS, fmt.Sprintf("%s/*.sql", "seed"))
	if err != nil {
		return fmt.Errorf("failed to read seed files, err: %w", err)
	}

	sort.Strings(filenames)

	for _, filename := range filenames {
		buf, err := seedFS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read seed file, filename=%s err=%w", filename, err)
		}
		stmt := string(buf)
		if err := db.execute(ctx, stmt); err != nil {
			return fmt.Errorf("seed error: statement:%s err=%w", stmt, err)
		}
	}
	return nil
}

// execute runs SQL statements. For MySQL/PostgreSQL, splits on semicolons.
func (db *DB) execute(ctx context.Context, stmt string) error {
	if db.profile.Driver == "mysql" || db.profile.Driver == "postgres" {
		stmts := splitSQLStatements(stmt)
		for _, s := range stmts {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, err := db.DBInstance.ExecContext(ctx, s); err != nil {
				return fmt.Errorf("failed to execute statement %q, err: %w", s, err)
			}
		}
		return nil
	}

	tx, err := db.DBInstance.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to execute statement, err: %w", err)
	}

	return tx.Commit()
}

// splitSQLStatements splits a SQL script into individual statements by semicolon,
// ignoring semicolons inside string literals.
func splitSQLStatements(script string) []string {
	var stmts []string
	var current strings.Builder
	inString := false
	var stringChar rune

	for _, ch := range script {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar {
				inString = false
			}
		} else {
			switch ch {
			case '\'', '"', '`':
				inString = true
				stringChar = ch
				current.WriteRune(ch)
			case ';':
				s := strings.TrimSpace(current.String())
				if s != "" {
					stmts = append(stmts, s)
				}
				current.Reset()
			default:
				current.WriteRune(ch)
			}
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// minorDirRegexp is a regular expression for minor version directory.
var minorDirRegexp = regexp.MustCompile(`^migration/prod/[0-9]+\.[0-9]+$`)

func getMinorVersionList() []string {
	minorVersionList := []string{}

	if err := fs.WalkDir(migrationFS, "migration", func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file.IsDir() && minorDirRegexp.MatchString(path) {
			minorVersionList = append(minorVersionList, file.Name())
		}

		return nil
	}); err != nil {
		panic(err)
	}

	sort.Sort(version.SortVersion(minorVersionList))

	return minorVersionList
}
