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
	query := "DELETE FROM memo WHERE id = ? AND creator_id = ?"

	require.Equal(t, "DELETE FROM memo WHERE id = $1 AND creator_id = $2", formatQuery("postgres", query))
	require.Equal(t, query, formatQuery("sqlite", query))
	require.Equal(t, query, formatQuery("mysql", query))
}
