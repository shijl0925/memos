package db

import (
"context"
"database/sql"
"fmt"
"strings"
)

type MigrationHistory struct {
Version   string
CreatedTs int64
}

type MigrationHistoryUpsert struct {
Version string
}

type MigrationHistoryFind struct {
Version *string
}

func (db *DB) FindMigrationHistoryList(ctx context.Context, find *MigrationHistoryFind) ([]*MigrationHistory, error) {
tx, err := db.DBInstance.BeginTx(ctx, nil)
if err != nil {
return nil, err
}
defer tx.Rollback()

list, err := findMigrationHistoryList(ctx, tx, db.profile.Driver, find)
if err != nil {
return nil, err
}

return list, nil
}

func (db *DB) UpsertMigrationHistory(ctx context.Context, upsert *MigrationHistoryUpsert) (*MigrationHistory, error) {
tx, err := db.DBInstance.BeginTx(ctx, nil)
if err != nil {
return nil, err
}
defer tx.Rollback()

migrationHistory, err := upsertMigrationHistory(ctx, tx, db.profile.Driver, upsert)
if err != nil {
return nil, err
}

if err := tx.Commit(); err != nil {
return nil, err
}

return migrationHistory, nil
}

func findMigrationHistoryList(ctx context.Context, tx *sql.Tx, driver string, find *MigrationHistoryFind) ([]*MigrationHistory, error) {
where, args := []string{"1 = 1"}, []any{}

if v := find.Version; v != nil {
where, args = append(where, "version = ?"), append(args, *v)
}

query := dbFormatQuery(driver, `SELECT version, created_ts FROM migration_history WHERE `+strings.Join(where, " AND ")+` ORDER BY version DESC`)
rows, err := tx.QueryContext(ctx, query, args...)
if err != nil {
return nil, err
}
defer rows.Close()

migrationHistoryList := make([]*MigrationHistory, 0)
for rows.Next() {
var migrationHistory MigrationHistory
if err := rows.Scan(
&migrationHistory.Version,
&migrationHistory.CreatedTs,
); err != nil {
return nil, err
}

migrationHistoryList = append(migrationHistoryList, &migrationHistory)
}

if err := rows.Err(); err != nil {
return nil, err
}

return migrationHistoryList, nil
}

func upsertMigrationHistory(ctx context.Context, tx *sql.Tx, driver string, upsert *MigrationHistoryUpsert) (*MigrationHistory, error) {
if driver == "mysql" {
_, err := tx.ExecContext(ctx, `INSERT INTO migration_history (version) VALUES (?) ON DUPLICATE KEY UPDATE version=VALUES(version)`, upsert.Version)
if err != nil {
return nil, err
}
list, err := findMigrationHistoryList(ctx, tx, driver, &MigrationHistoryFind{Version: &upsert.Version})
if err != nil {
return nil, err
}
if len(list) == 0 {
return nil, fmt.Errorf("migration history not found after upsert")
}
return list[0], nil
}

query := dbFormatQuery(driver, `INSERT INTO migration_history (version) VALUES (?) ON CONFLICT(version) DO UPDATE SET version=EXCLUDED.version RETURNING version, created_ts`)
var migrationHistory MigrationHistory
if err := tx.QueryRowContext(ctx, query, upsert.Version).Scan(
&migrationHistory.Version,
&migrationHistory.CreatedTs,
); err != nil {
return nil, err
}

return &migrationHistory, nil
}

// dbFormatQuery converts ? placeholders to $1, $2, ... for PostgreSQL.
func dbFormatQuery(driver, query string) string {
if driver != "postgres" {
return query
}
var b strings.Builder
idx := 1
for _, ch := range query {
if ch == '?' {
fmt.Fprintf(&b, "$%d", idx)
idx++
} else {
b.WriteRune(ch)
}
}
return b.String()
}
