package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/usememos/memos/api"
)

// memoRelationRaw is the store model for a MemoRelation.
// Fields have exactly the same meanings as api.MemoRelation.
type memoRelationRaw struct {
	MemoID        int
	RelatedMemoID int
	Type          api.MemoRelationType
}

func (raw *memoRelationRaw) toMemoRelation() *api.MemoRelation {
	return &api.MemoRelation{
		MemoID:        raw.MemoID,
		RelatedMemoID: raw.RelatedMemoID,
		Type:          raw.Type,
	}
}

// UpsertMemoRelation inserts or replaces a memo relation.
func (s *Store) UpsertMemoRelation(ctx context.Context, upsert *api.MemoRelation) (*api.MemoRelation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	raw, err := upsertMemoRelation(ctx, tx, s.driver, upsert)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, FormatError(err)
	}

	return raw.toMemoRelation(), nil
}

// FindMemoRelationList returns a list of memo relations matching the find criteria.
func (s *Store) FindMemoRelationList(ctx context.Context, find *api.MemoRelationFind) ([]*api.MemoRelation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	rawList, err := findMemoRelationList(ctx, tx, s.driver, find)
	if err != nil {
		return nil, err
	}

	list := make([]*api.MemoRelation, 0, len(rawList))
	for _, raw := range rawList {
		list = append(list, raw.toMemoRelation())
	}
	return list, nil
}

// DeleteMemoRelation removes memo relations matching the delete criteria.
func (s *Store) DeleteMemoRelation(ctx context.Context, delete *api.MemoRelationDelete) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FormatError(err)
	}
	defer tx.Rollback()

	if err := deleteMemoRelation(ctx, tx, s.driver, delete); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return FormatError(err)
	}

	return nil
}

func upsertMemoRelation(ctx context.Context, tx *sql.Tx, driver string, upsert *api.MemoRelation) (*memoRelationRaw, error) {
	if driver == "mysql" {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO memo_relation (memo_id, related_memo_id, type) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE type=VALUES(type)`,
			upsert.MemoID, upsert.RelatedMemoID, upsert.Type)
		if err != nil {
			return nil, FormatError(err)
		}
		raw := &memoRelationRaw{}
		row := tx.QueryRowContext(ctx, `SELECT memo_id, related_memo_id, type FROM memo_relation WHERE memo_id=? AND related_memo_id=? AND type=?`,
			upsert.MemoID, upsert.RelatedMemoID, upsert.Type)
		if err := row.Scan(&raw.MemoID, &raw.RelatedMemoID, &raw.Type); err != nil {
			return nil, FormatError(err)
		}
		return raw, nil
	}
	stmt := formatQuery(driver, `
		INSERT INTO memo_relation (memo_id, related_memo_id, type)
		VALUES (?, ?, ?)
		ON CONFLICT (memo_id, related_memo_id, type) DO UPDATE SET type = EXCLUDED.type
		RETURNING memo_id, related_memo_id, type
	`)
	raw := &memoRelationRaw{}
	if err := tx.QueryRowContext(ctx, stmt, upsert.MemoID, upsert.RelatedMemoID, upsert.Type).Scan(
		&raw.MemoID,
		&raw.RelatedMemoID,
		&raw.Type,
	); err != nil {
		return nil, FormatError(err)
	}
	return raw, nil
}

func findMemoRelationList(ctx context.Context, tx *sql.Tx, driver string, find *api.MemoRelationFind) ([]*memoRelationRaw, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.MemoID != nil {
		where, args = append(where, "memo_id = ?"), append(args, *find.MemoID)
	}
	if find.RelatedMemoID != nil {
		where, args = append(where, "related_memo_id = ?"), append(args, *find.RelatedMemoID)
	}
	if find.Type != nil {
		where, args = append(where, "type = ?"), append(args, string(*find.Type))
	}

	q := formatQuery(driver, `SELECT memo_id, related_memo_id, type FROM memo_relation WHERE `+strings.Join(where, " AND "))
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, FormatError(err)
	}
	defer rows.Close()

	rawList := make([]*memoRelationRaw, 0)
	for rows.Next() {
		raw := &memoRelationRaw{}
		if err := rows.Scan(&raw.MemoID, &raw.RelatedMemoID, &raw.Type); err != nil {
			return nil, FormatError(err)
		}
		rawList = append(rawList, raw)
	}
	if err := rows.Err(); err != nil {
		return nil, FormatError(err)
	}
	return rawList, nil
}

func deleteMemoRelation(ctx context.Context, tx *sql.Tx, driver string, delete *api.MemoRelationDelete) error {
	where, args := []string{"1 = 1"}, []any{}
	if delete.MemoID != nil {
		where, args = append(where, "memo_id = ?"), append(args, *delete.MemoID)
	}
	if delete.RelatedMemoID != nil {
		where, args = append(where, "related_memo_id = ?"), append(args, *delete.RelatedMemoID)
	}
	if delete.Type != nil {
		where, args = append(where, "type = ?"), append(args, string(*delete.Type))
	}
	stmt := `DELETE FROM memo_relation WHERE ` + strings.Join(where, " AND ")
	stmt = formatQuery(driver, stmt)
	if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
		return FormatError(err)
	}
	return nil
}

// findMemoRelationListMap returns memo relations for a set of memoIDs in a single query,
// keyed by memoID. This avoids N+1 queries when building a list of memos.
func findMemoRelationListMap(ctx context.Context, tx *sql.Tx, driver string, memoIDList []int) (map[int][]*api.MemoRelation, error) {
	relationListMap := make(map[int][]*api.MemoRelation, len(memoIDList))
	if len(memoIDList) == 0 {
		return relationListMap, nil
	}

	placeholder := make([]string, 0, len(memoIDList))
	args := make([]any, 0, len(memoIDList))
	for _, id := range memoIDList {
		placeholder = append(placeholder, "?")
		args = append(args, id)
	}

	q := formatQuery(driver, `SELECT memo_id, related_memo_id, type FROM memo_relation WHERE memo_id IN (`+strings.Join(placeholder, ", ")+`) ORDER BY memo_id`)
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, FormatError(err)
	}
	defer rows.Close()

	for rows.Next() {
		raw := &memoRelationRaw{}
		if err := rows.Scan(&raw.MemoID, &raw.RelatedMemoID, &raw.Type); err != nil {
			return nil, FormatError(err)
		}
		relationListMap[raw.MemoID] = append(relationListMap[raw.MemoID], raw.toMemoRelation())
	}
	if err := rows.Err(); err != nil {
		return nil, FormatError(err)
	}
	return relationListMap, nil
}

func vacuumMemoRelations(ctx context.Context, tx *sql.Tx, _ string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM memo_relation WHERE memo_id NOT IN (SELECT id FROM memo) OR related_memo_id NOT IN (SELECT id FROM memo)`)
	return err
}
