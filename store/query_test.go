package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoolLiteral(t *testing.T) {
	require.Equal(t, "TRUE", boolLiteral("postgres", true))
	require.Equal(t, "FALSE", boolLiteral("postgres", false))
	require.Equal(t, "1", boolLiteral("sqlite", true))
	require.Equal(t, "0", boolLiteral("sqlite", false))
	require.Equal(t, "1", boolLiteral("mysql", true))
	require.Equal(t, "0", boolLiteral("mysql", false))
}

func TestFormatQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		postgres string
	}{
		{
			name:     "delete with multiple predicates",
			query:    "DELETE FROM memo WHERE id = ? AND creator_id = ?",
			postgres: "DELETE FROM memo WHERE id = $1 AND creator_id = $2",
		},
		{
			name:     "delete with dynamically joined predicates",
			query:    "DELETE FROM tag WHERE name = ? AND creator_id = ?",
			postgres: "DELETE FROM tag WHERE name = $1 AND creator_id = $2",
		},
		{
			name: "select with multiline predicate",
			query: `
		SELECT
			id,
			memo_id,
			user_id,
			pinned
		FROM memo_organizer
		WHERE memo_id = ? AND user_id = ?
	`,
			postgres: `
		SELECT
			id,
			memo_id,
			user_id,
			pinned
		FROM memo_organizer
		WHERE memo_id = $1 AND user_id = $2
	`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.postgres, formatQuery("postgres", test.query))
			require.Equal(t, test.query, formatQuery("sqlite", test.query))
			require.Equal(t, test.query, formatQuery("mysql", test.query))
		})
	}
}
