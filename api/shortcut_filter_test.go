package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyShortcutFilter(t *testing.T) {
	find := &MemoFind{}
	err := ApplyShortcutFilter(find, `tag in ["tag1", "tag2"] && content.contains("Hello") && visibility in ["PUBLIC", "PRIVATE"] && has_link && has_task_list && has_code && pinned && created_ts >= 1777248000000 && created_ts < 1777507200000`)
	require.NoError(t, err)

	require.Equal(t, []string{"tag1", "tag2"}, find.TagSearchList)
	require.Equal(t, []string{"Hello"}, find.ContentContainsList)
	require.Equal(t, []Visibility{Public, Private}, find.VisibilityList)
	require.NotNil(t, find.HasLink)
	require.True(t, *find.HasLink)
	require.NotNil(t, find.HasTaskList)
	require.True(t, *find.HasTaskList)
	require.NotNil(t, find.HasCode)
	require.True(t, *find.HasCode)
	require.NotNil(t, find.Pinned)
	require.True(t, *find.Pinned)
	require.NotNil(t, find.CreatedTsAfter)
	require.Equal(t, int64(1777248000), *find.CreatedTsAfter)
	require.NotNil(t, find.CreatedTsBefore)
	require.Equal(t, int64(1777507200), *find.CreatedTsBefore)
}

func TestApplyShortcutFilterRejectsInvalidExpression(t *testing.T) {
	err := ApplyShortcutFilter(&MemoFind{}, `content.contains(xxx)`)
	require.Error(t, err)
}
