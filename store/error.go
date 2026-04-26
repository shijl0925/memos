package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/usememos/memos/common"
)

func FormatError(err error) error {
	if err == nil {
		return nil
	}

	switch err {
	case sql.ErrNoRows:
		return errors.New("data not found")
	default:
		if isUniqueConstraintError(err) {
			return common.Errorf(common.Conflict, fmt.Errorf("record already exists"))
		}
		return err
	}
}

// isUniqueConstraintError reports whether err is a unique/duplicate-key
// constraint violation from SQLite, MySQL, or PostgreSQL.
func isUniqueConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062 // ER_DUP_ENTRY
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}

	return false
}
