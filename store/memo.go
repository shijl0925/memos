package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

// memoRaw is the store model for an Memo.
// Fields have exactly the same meanings as Memo.
type memoRaw struct {
	ID int

	// Standard fields
	RowStatus api.RowStatus
	CreatorID int
	CreatedTs int64
	UpdatedTs int64

	// Domain specific fields
	Content    string
	Visibility api.Visibility
	Pinned     bool
}

// toMemo creates an instance of Memo based on the memoRaw.
// This is intended to be called when we need to compose an Memo relationship.
func (raw *memoRaw) toMemo() *api.Memo {
	return &api.Memo{
		ID: raw.ID,

		// Standard fields
		RowStatus: raw.RowStatus,
		CreatorID: raw.CreatorID,
		CreatedTs: raw.CreatedTs,
		UpdatedTs: raw.UpdatedTs,

		// Domain specific fields
		Content:    raw.Content,
		Visibility: raw.Visibility,
		Pinned:     raw.Pinned,

		// DisplayTs defaults to CreatedTs (can be overridden via patch)
		DisplayTs: raw.CreatedTs,
	}
}

func (s *Store) composeMemo(ctx context.Context, memo *api.Memo) (*api.Memo, error) {
	if err := s.composeMemoCreator(ctx, memo); err != nil {
		return nil, err
	}
	if err := s.composeMemoResourceList(ctx, memo); err != nil {
		return nil, err
	}
	if err := s.composeMemoRelationList(ctx, memo); err != nil {
		return nil, err
	}

	return memo, nil
}

func (s *Store) composeMemoRelationList(ctx context.Context, memo *api.Memo) error {
	relationList, err := s.FindMemoRelationList(ctx, &api.MemoRelationFind{MemoID: &memo.ID})
	if err != nil {
		return err
	}
	memo.RelationList = relationList
	return nil
}

func (s *Store) CreateMemo(ctx context.Context, create *api.MemoCreate) (*api.Memo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	memoRaw, err := createMemoRaw(ctx, tx, s.driver, create)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, FormatError(err)
	}

	s.memoCache.Store(memoRaw.ID, memoRaw)
	memo, err := s.composeMemo(ctx, memoRaw.toMemo())
	if err != nil {
		return nil, err
	}

	return memo, nil
}

func (s *Store) PatchMemo(ctx context.Context, patch *api.MemoPatch) (*api.Memo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	memoRaw, err := patchMemoRaw(ctx, tx, s.driver, patch)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, FormatError(err)
	}

	s.memoCache.Store(memoRaw.ID, memoRaw)
	memo, err := s.composeMemo(ctx, memoRaw.toMemo())
	if err != nil {
		return nil, err
	}

	return memo, nil
}

func (s *Store) FindMemoList(ctx context.Context, find *api.MemoFind) ([]*api.Memo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	memoRawList, err := findMemoRawList(ctx, tx, s.driver, find)
	if err != nil {
		return nil, err
	}

	return s.composeMemoList(ctx, tx, s.driver, memoRawList)
}

func (s *Store) FindMemo(ctx context.Context, find *api.MemoFind) (*api.Memo, error) {
	if find.ID != nil {
		if memo, ok := s.memoCache.Load(*find.ID); ok {
			memoRaw := memo.(*memoRaw)
			memo, err := s.composeMemo(ctx, memoRaw.toMemo())
			if err != nil {
				return nil, err
			}
			return memo, nil
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	list, err := findMemoRawList(ctx, tx, s.driver, find)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, &common.Error{Code: common.NotFound, Err: fmt.Errorf("not found")}
	}

	memoRaw := list[0]
	s.memoCache.Store(memoRaw.ID, memoRaw)
	memo, err := s.composeMemo(ctx, memoRaw.toMemo())
	if err != nil {
		return nil, err
	}

	return memo, nil
}

func (s *Store) DeleteMemo(ctx context.Context, delete *api.MemoDelete) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FormatError(err)
	}
	defer tx.Rollback()

	if err := deleteMemo(ctx, tx, delete); err != nil {
		return FormatError(err)
	}
	if err := vacuum(ctx, tx, s.driver); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return FormatError(err)
	}

	s.memoCache.Delete(delete.ID)
	return nil
}

func createMemoRaw(ctx context.Context, tx *sql.Tx, driver string, create *api.MemoCreate) (*memoRaw, error) {
	set := []string{"creator_id", "content", "visibility"}
	args := []any{create.CreatorID, create.Content, create.Visibility}
	placeholder := []string{"?", "?", "?"}

	if v := create.CreatedTs; v != nil {
		set, args, placeholder = append(set, "created_ts"), append(args, *v), append(placeholder, "?")
	}

	if driver == "mysql" {
		insertQuery := `INSERT INTO memo (` + strings.Join(set, ", ") + `) VALUES (` + strings.Join(placeholder, ",") + `)`
		result, err := tx.ExecContext(ctx, insertQuery, args...)
		if err != nil {
			return nil, FormatError(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, FormatError(err)
		}
		id32 := int(id)
		list, err := findMemoRawList(ctx, tx, driver, &api.MemoFind{ID: &id32})
		if err != nil {
			return nil, err
		}
		return list[0], nil
	}

	query := formatQuery(driver, `
		INSERT INTO memo (
			`+strings.Join(set, ", ")+`
		)
		VALUES (`+strings.Join(placeholder, ",")+`)
		RETURNING id, creator_id, created_ts, updated_ts, row_status, content, visibility
	`)
	var memoRaw memoRaw
	if err := tx.QueryRowContext(ctx, query, args...).Scan(
		&memoRaw.ID,
		&memoRaw.CreatorID,
		&memoRaw.CreatedTs,
		&memoRaw.UpdatedTs,
		&memoRaw.RowStatus,
		&memoRaw.Content,
		&memoRaw.Visibility,
	); err != nil {
		return nil, FormatError(err)
	}

	return &memoRaw, nil
}

func patchMemoRaw(ctx context.Context, tx *sql.Tx, driver string, patch *api.MemoPatch) (*memoRaw, error) {
	set, args := []string{}, []any{}

	if v := patch.CreatedTs; v != nil {
		set, args = append(set, "created_ts = ?"), append(args, *v)
	}
	if v := patch.UpdatedTs; v != nil {
		set, args = append(set, "updated_ts = ?"), append(args, *v)
	}
	if v := patch.RowStatus; v != nil {
		set, args = append(set, "row_status = ?"), append(args, *v)
	}
	if v := patch.Content; v != nil {
		set, args = append(set, "content = ?"), append(args, *v)
	}
	if v := patch.Visibility; v != nil {
		set, args = append(set, "visibility = ?"), append(args, *v)
	}

	args = append(args, patch.ID)

	if driver == "mysql" {
		stmt := `UPDATE memo SET ` + strings.Join(set, ", ") + ` WHERE id = ?`
		if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
			return nil, FormatError(err)
		}
		list, err := findMemoRawList(ctx, tx, driver, &api.MemoFind{ID: &patch.ID})
		if err != nil {
			return nil, err
		}
		return list[0], nil
	}

	query := formatQuery(driver, `
		UPDATE memo
		SET `+strings.Join(set, ", ")+`
		WHERE id = ?
		RETURNING id, creator_id, created_ts, updated_ts, row_status, content, visibility
	`)
	var memoRaw memoRaw
	if err := tx.QueryRowContext(ctx, query, args...).Scan(
		&memoRaw.ID,
		&memoRaw.CreatorID,
		&memoRaw.CreatedTs,
		&memoRaw.UpdatedTs,
		&memoRaw.RowStatus,
		&memoRaw.Content,
		&memoRaw.Visibility,
	); err != nil {
		return nil, FormatError(err)
	}

	return &memoRaw, nil
}

func findMemoRawList(ctx context.Context, tx *sql.Tx, driver string, find *api.MemoFind) ([]*memoRaw, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.ID; v != nil {
		where, args = append(where, "memo.id = ?"), append(args, *v)
	}
	if v := find.CreatorID; v != nil {
		where, args = append(where, "memo.creator_id = ?"), append(args, *v)
	}
	if v := find.RowStatus; v != nil {
		where, args = append(where, "memo.row_status = ?"), append(args, *v)
	}
	if v := find.Pinned; v != nil {
		where = append(where, "memo_organizer.pinned = 1")
	}
	if v := find.ContentSearch; v != nil {
		where, args = append(where, "memo.content LIKE ?"), append(args, "%"+*v+"%")
	}
	if v := find.CreatedTsAfter; v != nil {
		where, args = append(where, "memo.created_ts >= ?"), append(args, *v)
	}
	if v := find.CreatedTsBefore; v != nil {
		where, args = append(where, "memo.created_ts < ?"), append(args, *v)
	}
	if v := find.VisibilityList; len(v) != 0 {
		placeholders := make([]string, 0, len(v))
		for _, visibility := range v {
			placeholders = append(placeholders, "?")
			args = append(args, visibility)
		}
		where = append(where, fmt.Sprintf("memo.visibility in (%s)", strings.Join(placeholders, ",")))
	}

	query := formatQuery(driver, `
		SELECT
			memo.id,
			memo.creator_id,
			memo.created_ts,
			memo.updated_ts,
			memo.row_status,
			memo.content,
			memo.visibility,
			COALESCE(memo_organizer.pinned, 0) AS pinned
		FROM memo
		LEFT JOIN memo_organizer ON memo_organizer.memo_id = memo.id AND memo_organizer.user_id = memo.creator_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY pinned DESC, memo.created_ts DESC
	`)
	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, FormatError(err)
	}
	defer rows.Close()

	memoRawList := make([]*memoRaw, 0)
	for rows.Next() {
		var memoRaw memoRaw
		var pinned sql.NullBool
		if err := rows.Scan(
			&memoRaw.ID,
			&memoRaw.CreatorID,
			&memoRaw.CreatedTs,
			&memoRaw.UpdatedTs,
			&memoRaw.RowStatus,
			&memoRaw.Content,
			&memoRaw.Visibility,
			&pinned,
		); err != nil {
			return nil, FormatError(err)
		}

		if pinned.Valid {
			memoRaw.Pinned = pinned.Bool
		}
		memoRawList = append(memoRawList, &memoRaw)
	}

	if err := rows.Err(); err != nil {
		return nil, FormatError(err)
	}

	return memoRawList, nil
}

func deleteMemo(ctx context.Context, tx *sql.Tx, delete *api.MemoDelete) error {
	where, args := []string{"id = ?"}, []any{delete.ID}

	stmt := `DELETE FROM memo WHERE ` + strings.Join(where, " AND ")
	result, err := tx.ExecContext(ctx, stmt, args...)
	if err != nil {
		return FormatError(err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &common.Error{Code: common.NotFound, Err: fmt.Errorf("memo not found")}
	}

	return nil
}

func (_ *Store) composeMemoList(ctx context.Context, tx *sql.Tx, driver string, memoRawList []*memoRaw) ([]*api.Memo, error) {
	list := make([]*api.Memo, 0, len(memoRawList))
	if len(memoRawList) == 0 {
		return list, nil
	}

	creatorIDSet := make(map[int]struct{}, len(memoRawList))
	memoIDList := make([]int, 0, len(memoRawList))
	for _, raw := range memoRawList {
		creatorIDSet[raw.CreatorID] = struct{}{}
		memoIDList = append(memoIDList, raw.ID)
	}

	creatorIDList := make([]int, 0, len(creatorIDSet))
	for creatorID := range creatorIDSet {
		creatorIDList = append(creatorIDList, creatorID)
	}

	userMap, err := findUserRawMapByIDList(ctx, tx, driver, creatorIDList)
	if err != nil {
		return nil, err
	}
	resourceListMap, err := findMemoResourceListMap(ctx, tx, driver, memoIDList)
	if err != nil {
		return nil, err
	}

	// Build a relation map keyed by memoID using a single batch query.
	relationListMap, err := findMemoRelationListMap(ctx, tx, driver, memoIDList)
	if err != nil {
		return nil, err
	}

	for _, raw := range memoRawList {
		memo := raw.toMemo()
		user, ok := userMap[memo.CreatorID]
		if !ok {
			return nil, &common.Error{Code: common.NotFound, Err: fmt.Errorf("not found user with id %d", memo.CreatorID)}
		}
		if user.Nickname != "" {
			memo.CreatorName = user.Nickname
		} else {
			memo.CreatorName = user.Username
		}
		memo.CreatorUsername = user.Username

		if resourceList, ok := resourceListMap[memo.ID]; ok {
			memo.ResourceList = resourceList
		} else {
			memo.ResourceList = []*api.Resource{}
		}

		if relationList, ok := relationListMap[memo.ID]; ok {
			memo.RelationList = relationList
		} else {
			memo.RelationList = []*api.MemoRelation{}
		}

		list = append(list, memo)
	}

	return list, nil
}

func vacuumMemo(ctx context.Context, tx *sql.Tx, driver string) error {
	stmt := `DELETE FROM memo WHERE creator_id NOT IN (SELECT id FROM ` + userTableName(driver) + `)`
	_, err := tx.ExecContext(ctx, stmt)
	if err != nil {
		return FormatError(err)
	}

	return nil
}
