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
