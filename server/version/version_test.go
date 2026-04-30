package version

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCurrentVersion(t *testing.T) {
	require.Equal(t, DevVersion, GetCurrentVersion("dev"))
	require.Equal(t, Version, GetCurrentVersion("prod"))
	require.Equal(t, Version, GetCurrentVersion("unknown"))
}

func TestGetMinorAndSchemaVersion(t *testing.T) {
	require.Equal(t, "1.2", GetMinorVersion("1.2.3"))
	require.Empty(t, GetMinorVersion("1.2"))
	require.Equal(t, "1.2.0", GetSchemaVersion("1.2.3"))
}

func TestIsVersionGreaterOrEqualThan(t *testing.T) {
	tests := []struct {
		version string
		target  string
		want    bool
	}{
		{
			version: "0.9.1",
			target:  "0.9.1",
			want:    true,
		},
		{
			version: "0.10.0",
			target:  "0.9.1",
			want:    true,
		},
		{
			version: "0.9.0",
			target:  "0.9.1",
			want:    false,
		},
	}
	for _, test := range tests {
		result := IsVersionGreaterOrEqualThan(test.version, test.target)
		if result != test.want {
			t.Errorf("got result %v, want %v.", result, test.want)
		}
	}
}

func TestIsVersionGreaterThan(t *testing.T) {
	tests := []struct {
		version string
		target  string
		want    bool
	}{
		{
			version: "0.9.1",
			target:  "0.9.1",
			want:    false,
		},
		{
			version: "0.10.0",
			target:  "0.8.0",
			want:    true,
		},
		{
			version: "0.8.0",
			target:  "0.10.0",
			want:    false,
		},
		{
			version: "0.9.0",
			target:  "0.9.1",
			want:    false,
		},
	}
	for _, test := range tests {
		result := IsVersionGreaterThan(test.version, test.target)
		if result != test.want {
			t.Errorf("got result %v, want %v.", result, test.want)
		}
	}
}

func TestSortVersion(t *testing.T) {
	tests := []struct {
		versionList []string
		want        []string
	}{
		{
			versionList: []string{"0.9.1", "0.10.0", "0.8.0"},
			want:        []string{"0.8.0", "0.9.1", "0.10.0"},
		},
		{
			versionList: []string{"1.9.1", "0.9.1", "0.10.0", "0.8.0"},
			want:        []string{"0.8.0", "0.9.1", "0.10.0", "1.9.1"},
		},
	}
	for _, test := range tests {
		sort.Sort(SortVersion(test.versionList))
		assert.Equal(t, test.versionList, test.want)
	}
}
