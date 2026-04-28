package store

import (
	"context"
	"database/sql"
	"sync"

	"github.com/usememos/memos/server/profile"
)

// Store provides database access to all raw objects.
type Store struct {
	db      *sql.DB
	profile *profile.Profile
	driver  string

	userCache        sync.Map // map[int]*userRaw
	userSettingCache sync.Map // map[string]*userSettingRaw
	memoCache        sync.Map // map[int]*memoRaw
	shortcutCache    sync.Map // map[int]*shortcutRaw
	idpCache         sync.Map // map[int]*identityProviderMessage
}

// New creates a new instance of Store.
func New(db *sql.DB, profile *profile.Profile) *Store {
	return &Store{
		db:      db,
		profile: profile,
		driver:  profile.Driver,
	}
}

func (s *Store) Vacuum(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FormatError(err)
	}
	defer tx.Rollback()

	if err := vacuum(ctx, tx, s.driver); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return FormatError(err)
	}

	if s.driver == "sqlite3" {
		if _, err := s.db.Exec("VACUUM"); err != nil {
			return err
		}
	}

	return nil
}

// vacuum cleans up orphaned records in a transaction.
func vacuum(ctx context.Context, tx *sql.Tx, driver string) error {
	if err := vacuumMemo(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumResource(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumShortcut(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumUserSetting(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumMemoOrganizer(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumMemoResource(ctx, tx, driver); err != nil {
		return err
	}
	if err := vacuumMemoRelations(ctx, tx, driver); err != nil {
		return err
	}
	return vacuumTag(ctx, tx, driver)
}
