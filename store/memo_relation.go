package store

import (
	"context"
	"database/sql"
	"strings"
)

// MemoRelationType is the type of a memo relation.
type MemoRelationType string

const (
	// MemoRelationReference means the memo references another memo.
	MemoRelationReference MemoRelationType = "REFERENCE"
	// MemoRelationAdditional means the memo is additional context for another memo.
	MemoRelationAdditional MemoRelationType = "ADDITIONAL"
)

// MemoRelation represents a relationship between two memos.
type MemoRelation struct {
	MemoID        int
	RelatedMemoID int
	Type          MemoRelationType
}

// MemoRelationFind is used to query memo relations.
type MemoRelationFind struct {
	MemoID        *int
	RelatedMemoID *int
	Type          *MemoRelationType
}

// MemoRelationDelete is used to delete memo relations.
type MemoRelationDelete struct {
	MemoID        *int
	RelatedMemoID *int
	Type          *MemoRelationType
}

// UpsertMemoRelation inserts or replaces a memo relation.
func (s *Store) UpsertMemoRelation(ctx context.Context, create *MemoRelation) (*MemoRelation, error) {
	stmt := `
		INSERT INTO memo_relation (
			memo_id,
			related_memo_id,
			type
		)
		VALUES (?, ?, ?)
		ON CONFLICT (memo_id, related_memo_id, type) DO UPDATE SET
			type = EXCLUDED.type
		RETURNING memo_id, related_memo_id, type
	`
	relation := &MemoRelation{}
	if err := s.db.QueryRowContext(ctx, stmt, create.MemoID, create.RelatedMemoID, create.Type).Scan(
		&relation.MemoID,
		&relation.RelatedMemoID,
		&relation.Type,
	); err != nil {
		return nil, err
	}
	return relation, nil
}

// FindMemoRelationList returns a list of memo relations matching the find criteria.
func (s *Store) FindMemoRelationList(ctx context.Context, find *MemoRelationFind) ([]*MemoRelation, error) {
	where, args := []string{"TRUE"}, []any{}
	if find.MemoID != nil {
		where, args = append(where, "memo_id = ?"), append(args, *find.MemoID)
	}
	if find.RelatedMemoID != nil {
		where, args = append(where, "related_memo_id = ?"), append(args, *find.RelatedMemoID)
	}
	if find.Type != nil {
		where, args = append(where, "type = ?"), append(args, string(*find.Type))
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT memo_id, related_memo_id, type
		FROM memo_relation
		WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*MemoRelation{}
	for rows.Next() {
		relation := &MemoRelation{}
		if err := rows.Scan(&relation.MemoID, &relation.RelatedMemoID, &relation.Type); err != nil {
			return nil, err
		}
		list = append(list, relation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteMemoRelation removes memo relations matching the delete criteria.
func (s *Store) DeleteMemoRelation(ctx context.Context, delete *MemoRelationDelete) error {
	where, args := []string{"TRUE"}, []any{}
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
	_, err := s.db.ExecContext(ctx, stmt, args...)
	return err
}

func vacuumMemoRelations(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM memo_relation
		WHERE memo_id NOT IN (SELECT id FROM memo)
		   OR related_memo_id NOT IN (SELECT id FROM memo)
	`)
	return err
}
