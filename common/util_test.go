package common

import (
	"errors"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{
			email: "t@gmail.com",
			want:  true,
		},
		{
			email: "@qq.com",
			want:  false,
		},
		{
			email: "1@gmail",
			want:  true,
		},
	}
	for _, test := range tests {
		result := ValidateEmail(test.email)
		if result != test.want {
			t.Errorf("Validate Email %s: got result %v, want %v.", test.email, result, test.want)
		}
	}
}

func TestHasPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		prefixes []string
		want     bool
	}{
		{
			name:     "matches one of many prefixes",
			src:      "resources/avatars/user.png",
			prefixes: []string{"assets/", "resources/"},
			want:     true,
		},
		{
			name:     "does not match any prefix",
			src:      "memos/1",
			prefixes: []string{"users/", "resources/"},
			want:     false,
		},
		{
			name:     "empty prefix list",
			src:      "memos/1",
			prefixes: nil,
			want:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, HasPrefixes(test.src, test.prefixes...))
		})
	}
}

func TestMin(t *testing.T) {
	require.Equal(t, 1, Min(1, 2))
	require.Equal(t, -2, Min(3, -2))
	require.Equal(t, 5, Min(5, 5))
}

func TestGenUUID(t *testing.T) {
	_, err := uuid.Parse(GenUUID())
	require.NoError(t, err)
}

func TestRandomString(t *testing.T) {
	empty, err := RandomString(0)
	require.NoError(t, err)
	require.Empty(t, empty)

	random, err := RandomString(64)
	require.NoError(t, err)
	require.Len(t, random, 64)
	require.True(t, regexp.MustCompile(`^[0-9a-zA-Z]+$`).MatchString(random))
}

func TestApplicationErrorHelpers(t *testing.T) {
	require.Equal(t, Ok, ErrorCode(nil))
	require.Empty(t, ErrorMessage(nil))

	appErr := Errorf(NotFound, errors.New("memo not found"))
	require.Equal(t, "memo not found", appErr.Error())
	require.Equal(t, NotFound, ErrorCode(appErr))
	require.Equal(t, "memo not found", ErrorMessage(appErr))

	wrappedErr := errors.Join(errors.New("outer"), appErr)
	require.Equal(t, NotFound, ErrorCode(wrappedErr))
	require.Equal(t, "memo not found", ErrorMessage(wrappedErr))

	plainErr := errors.New("database unavailable")
	require.Equal(t, Internal, ErrorCode(plainErr))
	require.Equal(t, "Internal error.", ErrorMessage(plainErr))
}
