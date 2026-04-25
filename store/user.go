package store

import (
"context"
"database/sql"
"fmt"
"strings"

"github.com/usememos/memos/api"
"github.com/usememos/memos/common"
)

// userRaw is the store model for an User.
// Fields have exactly the same meanings as User.
type userRaw struct {
ID int

// Standard fields
RowStatus api.RowStatus
CreatedTs int64
UpdatedTs int64

// Domain specific fields
Username     string
Role         api.Role
Email        string
Nickname     string
PasswordHash string
OpenID       string
AvatarURL    string
}

func (raw *userRaw) toUser() *api.User {
return &api.User{
ID: raw.ID,

RowStatus: raw.RowStatus,
CreatedTs: raw.CreatedTs,
UpdatedTs: raw.UpdatedTs,

Username:     raw.Username,
Role:         raw.Role,
Email:        raw.Email,
Nickname:     raw.Nickname,
PasswordHash: raw.PasswordHash,
OpenID:       raw.OpenID,
AvatarURL:    raw.AvatarURL,
}
}

func (s *Store) composeMemoCreator(ctx context.Context, memo *api.Memo) error {
user, err := s.FindUser(ctx, &api.UserFind{
ID: &memo.CreatorID,
})
if err != nil {
return err
}

if user.Nickname != "" {
memo.CreatorName = user.Nickname
} else {
memo.CreatorName = user.Username
}
memo.CreatorUsername = user.Username
return nil
}

func (s *Store) CreateUser(ctx context.Context, create *api.UserCreate) (*api.User, error) {
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, FormatError(err)
}
defer tx.Rollback()

userRaw, err := createUser(ctx, tx, s.driver, create)
if err != nil {
return nil, err
}

if err := tx.Commit(); err != nil {
return nil, FormatError(err)
}

s.userCache.Store(userRaw.ID, userRaw)
user := userRaw.toUser()
return user, nil
}

func (s *Store) PatchUser(ctx context.Context, patch *api.UserPatch) (*api.User, error) {
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, FormatError(err)
}
defer tx.Rollback()

userRaw, err := patchUser(ctx, tx, s.driver, patch)
if err != nil {
return nil, err
}

if err := tx.Commit(); err != nil {
return nil, FormatError(err)
}

s.userCache.Store(userRaw.ID, userRaw)
user := userRaw.toUser()
return user, nil
}

func (s *Store) FindUserList(ctx context.Context, find *api.UserFind) ([]*api.User, error) {
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, FormatError(err)
}
defer tx.Rollback()

userRawList, err := findUserList(ctx, tx, s.driver, find)
if err != nil {
return nil, err
}

list := []*api.User{}
for _, raw := range userRawList {
list = append(list, raw.toUser())
}

return list, nil
}

func (s *Store) FindUser(ctx context.Context, find *api.UserFind) (*api.User, error) {
if find.ID != nil {
if user, ok := s.userCache.Load(*find.ID); ok {
return user.(*userRaw).toUser(), nil
}
}

tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, FormatError(err)
}
defer tx.Rollback()

list, err := findUserList(ctx, tx, s.driver, find)
if err != nil {
return nil, err
}

if len(list) == 0 {
return nil, &common.Error{Code: common.NotFound, Err: fmt.Errorf("not found user with filter %+v", find)}
}

userRaw := list[0]
s.userCache.Store(userRaw.ID, userRaw)
user := userRaw.toUser()
return user, nil
}

func (s *Store) DeleteUser(ctx context.Context, delete *api.UserDelete) error {
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return FormatError(err)
}
defer tx.Rollback()

if err := deleteUser(ctx, tx, s.driver, delete); err != nil {
return err
}
if err := vacuum(ctx, tx, s.driver); err != nil {
return err
}

if err := tx.Commit(); err != nil {
return err
}

s.userCache.Delete(delete.ID)
return nil
}

func createUser(ctx context.Context, tx *sql.Tx, driver string, create *api.UserCreate) (*userRaw, error) {
tbl := userTableName(driver)
if driver == "mysql" {
result, err := tx.ExecContext(ctx, `INSERT INTO `+tbl+` (username, role, email, nickname, password_hash, open_id) VALUES (?, ?, ?, ?, ?, ?)`,
create.Username, create.Role, create.Email, create.Nickname, create.PasswordHash, create.OpenID)
if err != nil {
return nil, FormatError(err)
}
id, err := result.LastInsertId()
if err != nil {
return nil, FormatError(err)
}
idInt := int(id)
list, err := findUserList(ctx, tx, driver, &api.UserFind{ID: &idInt})
if err != nil {
return nil, err
}
return list[0], nil
}

query := formatQuery(driver, `INSERT INTO `+tbl+` (username, role, email, nickname, password_hash, open_id) VALUES (?, ?, ?, ?, ?, ?) RETURNING id, username, role, email, nickname, password_hash, open_id, avatar_url, created_ts, updated_ts, row_status`)
var userRaw userRaw
if err := tx.QueryRowContext(ctx, query,
create.Username, create.Role, create.Email, create.Nickname, create.PasswordHash, create.OpenID,
).Scan(
&userRaw.ID, &userRaw.Username, &userRaw.Role, &userRaw.Email, &userRaw.Nickname,
&userRaw.PasswordHash, &userRaw.OpenID, &userRaw.AvatarURL,
&userRaw.CreatedTs, &userRaw.UpdatedTs, &userRaw.RowStatus,
); err != nil {
return nil, FormatError(err)
}

return &userRaw, nil
}

func patchUser(ctx context.Context, tx *sql.Tx, driver string, patch *api.UserPatch) (*userRaw, error) {
set, args := []string{}, []any{}

if v := patch.UpdatedTs; v != nil {
set, args = append(set, "updated_ts = ?"), append(args, *v)
}
if v := patch.RowStatus; v != nil {
set, args = append(set, "row_status = ?"), append(args, *v)
}
if v := patch.Username; v != nil {
set, args = append(set, "username = ?"), append(args, *v)
}
if v := patch.Email; v != nil {
set, args = append(set, "email = ?"), append(args, *v)
}
if v := patch.Nickname; v != nil {
set, args = append(set, "nickname = ?"), append(args, *v)
}
if v := patch.AvatarURL; v != nil {
set, args = append(set, "avatar_url = ?"), append(args, *v)
}
if v := patch.PasswordHash; v != nil {
set, args = append(set, "password_hash = ?"), append(args, *v)
}
if v := patch.OpenID; v != nil {
set, args = append(set, "open_id = ?"), append(args, *v)
}

args = append(args, patch.ID)
tbl := userTableName(driver)

if driver == "mysql" {
if _, err := tx.ExecContext(ctx, `UPDATE `+tbl+` SET `+strings.Join(set, ", ")+` WHERE id = ?`, args...); err != nil {
return nil, FormatError(err)
}
list, err := findUserList(ctx, tx, driver, &api.UserFind{ID: &patch.ID})
if err != nil {
return nil, err
}
return list[0], nil
}

query := formatQuery(driver, `UPDATE `+tbl+` SET `+strings.Join(set, ", ")+` WHERE id = ? RETURNING id, username, role, email, nickname, password_hash, open_id, avatar_url, created_ts, updated_ts, row_status`)
var userRaw userRaw
if err := tx.QueryRowContext(ctx, query, args...).Scan(
&userRaw.ID, &userRaw.Username, &userRaw.Role, &userRaw.Email, &userRaw.Nickname,
&userRaw.PasswordHash, &userRaw.OpenID, &userRaw.AvatarURL,
&userRaw.CreatedTs, &userRaw.UpdatedTs, &userRaw.RowStatus,
); err != nil {
return nil, FormatError(err)
}

return &userRaw, nil
}

func findUserList(ctx context.Context, tx *sql.Tx, driver string, find *api.UserFind) ([]*userRaw, error) {
where, args := []string{"1 = 1"}, []any{}

if v := find.ID; v != nil {
where, args = append(where, "id = ?"), append(args, *v)
}
if v := find.Username; v != nil {
where, args = append(where, "username = ?"), append(args, *v)
}
if v := find.Role; v != nil {
where, args = append(where, "role = ?"), append(args, *v)
}
if v := find.Email; v != nil {
where, args = append(where, "email = ?"), append(args, *v)
}
if v := find.Nickname; v != nil {
where, args = append(where, "nickname = ?"), append(args, *v)
}
if v := find.OpenID; v != nil {
where, args = append(where, "open_id = ?"), append(args, *v)
}

tbl := userTableName(driver)
query := formatQuery(driver, `SELECT id, username, role, email, nickname, password_hash, open_id, avatar_url, created_ts, updated_ts, row_status FROM `+tbl+` WHERE `+strings.Join(where, " AND ")+` ORDER BY created_ts DESC, row_status DESC`)
rows, err := tx.QueryContext(ctx, query, args...)
if err != nil {
return nil, FormatError(err)
}
defer rows.Close()

userRawList := make([]*userRaw, 0)
for rows.Next() {
var userRaw userRaw
if err := rows.Scan(
&userRaw.ID, &userRaw.Username, &userRaw.Role, &userRaw.Email, &userRaw.Nickname,
&userRaw.PasswordHash, &userRaw.OpenID, &userRaw.AvatarURL,
&userRaw.CreatedTs, &userRaw.UpdatedTs, &userRaw.RowStatus,
); err != nil {
return nil, FormatError(err)
}
userRawList = append(userRawList, &userRaw)
}

if err := rows.Err(); err != nil {
return nil, FormatError(err)
}

return userRawList, nil
}

func findUserRawMapByIDList(ctx context.Context, tx *sql.Tx, driver string, idList []int) (map[int]*userRaw, error) {
userMap := make(map[int]*userRaw, len(idList))
if len(idList) == 0 {
return userMap, nil
}

wherePlaceholder := make([]string, 0, len(idList))
args := make([]any, 0, len(idList))
for _, id := range idList {
wherePlaceholder = append(wherePlaceholder, "?")
args = append(args, id)
}

tbl := userTableName(driver)
query := formatQuery(driver, `SELECT id, username, role, email, nickname, password_hash, open_id, avatar_url, created_ts, updated_ts, row_status FROM `+tbl+` WHERE id IN (`+strings.Join(wherePlaceholder, ", ")+`)`)
rows, err := tx.QueryContext(ctx, query, args...)
if err != nil {
return nil, FormatError(err)
}
defer rows.Close()

for rows.Next() {
var userRaw userRaw
if err := rows.Scan(
&userRaw.ID, &userRaw.Username, &userRaw.Role, &userRaw.Email, &userRaw.Nickname,
&userRaw.PasswordHash, &userRaw.OpenID, &userRaw.AvatarURL,
&userRaw.CreatedTs, &userRaw.UpdatedTs, &userRaw.RowStatus,
); err != nil {
return nil, FormatError(err)
}
userMap[userRaw.ID] = &userRaw
}

if err := rows.Err(); err != nil {
return nil, FormatError(err)
}

return userMap, nil
}

func deleteUser(ctx context.Context, tx *sql.Tx, driver string, delete *api.UserDelete) error {
tbl := userTableName(driver)
result, err := tx.ExecContext(ctx, `DELETE FROM `+tbl+` WHERE id = ?`, delete.ID)
if err != nil {
return FormatError(err)
}

rows, err := result.RowsAffected()
if err != nil {
return err
}
if rows == 0 {
return &common.Error{Code: common.NotFound, Err: fmt.Errorf("user not found")}
}

return nil
}
