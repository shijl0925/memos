package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/store"
	"golang.org/x/crypto/bcrypt"
)

func TestMemoStore(t *testing.T) {
	ctx := context.Background()
	store := NewTestingStore(ctx, t)
	user, err := createTestingHostUser(ctx, store)
	require.NoError(t, err)
	memoCreate := &api.MemoCreate{
		CreatorID:  user.ID,
		Content:    "test_content",
		Visibility: api.Public,
	}
	memo, err := store.CreateMemo(ctx, memoCreate)
	require.NoError(t, err)
	require.Equal(t, memoCreate.Content, memo.Content)
	memoPatchContent := "test_content_2"
	memoPatch := &api.MemoPatch{
		ID:      memo.ID,
		Content: &memoPatchContent,
	}
	memo, err = store.PatchMemo(ctx, memoPatch)
	require.NoError(t, err)
	require.Equal(t, memoPatchContent, memo.Content)
	memoList, err := store.FindMemoList(ctx, &api.MemoFind{
		CreatorID: &user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(memoList))
	require.Equal(t, memo, memoList[0])
	err = store.DeleteMemo(ctx, &api.MemoDelete{
		ID: memo.ID,
	})
	require.NoError(t, err)
}

func TestFindMemoListComposesCreatorsAndResourcesInBatch(t *testing.T) {
	ctx := context.Background()
	store := NewTestingStore(ctx, t)
	firstUser, err := createTestingHostUser(ctx, store)
	require.NoError(t, err)

	secondUser, err := createTestingUser(ctx, store, &api.UserCreate{
		Username: "user2",
		Role:     api.NormalUser,
		Email:    "user2@test.com",
		Nickname: "",
		Password: "user2_password",
		OpenID:   "user2_open_id",
	})
	require.NoError(t, err)

	firstCreatedTs := int64(1)
	secondCreatedTs := int64(2)
	firstMemo, err := store.CreateMemo(ctx, &api.MemoCreate{
		CreatorID:  firstUser.ID,
		CreatedTs:  &firstCreatedTs,
		Content:    "first memo",
		Visibility: api.Public,
	})
	require.NoError(t, err)
	secondMemo, err := store.CreateMemo(ctx, &api.MemoCreate{
		CreatorID:  secondUser.ID,
		CreatedTs:  &secondCreatedTs,
		Content:    "second memo",
		Visibility: api.Public,
	})
	require.NoError(t, err)

	firstResource, err := store.CreateResource(ctx, &api.ResourceCreate{
		CreatorID: firstUser.ID,
		Filename:  "first.png",
		Blob:      []byte("first"),
		Type:      "image/png",
		Size:      5,
	})
	require.NoError(t, err)
	secondResource, err := store.CreateResource(ctx, &api.ResourceCreate{
		CreatorID: firstUser.ID,
		Filename:  "second.png",
		Blob:      []byte("second"),
		Type:      "image/png",
		Size:      6,
	})
	require.NoError(t, err)

	_, err = store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
		MemoID:     firstMemo.ID,
		ResourceID: firstResource.ID,
	})
	require.NoError(t, err)
	_, err = store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
		MemoID:     firstMemo.ID,
		ResourceID: secondResource.ID,
	})
	require.NoError(t, err)
	_, err = store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
		MemoID:     secondMemo.ID,
		ResourceID: firstResource.ID,
	})
	require.NoError(t, err)

	memoList, err := store.FindMemoList(ctx, &api.MemoFind{})
	require.NoError(t, err)
	require.Len(t, memoList, 2)

	require.Equal(t, secondMemo.ID, memoList[0].ID)
	require.Equal(t, secondUser.Username, memoList[0].CreatorName)
	require.Len(t, memoList[0].ResourceList, 1)
	require.Equal(t, firstResource.ID, memoList[0].ResourceList[0].ID)
	require.Equal(t, 2, memoList[0].ResourceList[0].LinkedMemoAmount)

	require.Equal(t, firstMemo.ID, memoList[1].ID)
	require.Equal(t, firstUser.Nickname, memoList[1].CreatorName)
	require.Len(t, memoList[1].ResourceList, 2)
	require.Equal(t, firstResource.ID, memoList[1].ResourceList[0].ID)
	require.Equal(t, secondResource.ID, memoList[1].ResourceList[1].ID)
	require.Equal(t, 2, memoList[1].ResourceList[0].LinkedMemoAmount)
	require.Equal(t, 1, memoList[1].ResourceList[1].LinkedMemoAmount)
}

func createTestingUser(ctx context.Context, store *store.Store, userCreate *api.UserCreate) (*api.User, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	userCreate.PasswordHash = string(passwordHash)
	return store.CreateUser(ctx, userCreate)
}
