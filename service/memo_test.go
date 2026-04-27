package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/server/profile"
	teststore "github.com/usememos/memos/test/store"
	"golang.org/x/crypto/bcrypt"
)

func newTestService(ctx context.Context, t *testing.T) *Service {
	t.Helper()
	st := teststore.NewTestingStore(ctx, t)
	return New(st, &profile.Profile{})
}

// createTestHostUser seeds the store with a Host user, bypassing the service
// layer so that tests have a valid actor without needing a prior sign-up.
func createTestHostUser(ctx context.Context, svc *Service, t *testing.T) *api.User {
	t.Helper()
	return createTestUser(ctx, svc, t, "host", api.Host)
}

func createTestUser(ctx context.Context, svc *Service, t *testing.T, username string, role api.Role) *api.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("test_password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	user, err := svc.Store.CreateUser(ctx, &api.UserCreate{
		Username:     username,
		Role:         role,
		Nickname:     username + "_nickname",
		Email:        username + "@test.com",
		OpenID:       username + "-open-id",
		PasswordHash: string(hash),
	})
	require.NoError(t, err)
	return user
}

func TestCreateMemo_ContentLengthOverflow(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	user := createTestHostUser(ctx, svc, t)

	_, err := svc.CreateMemo(ctx, user.ID, &api.MemoCreate{
		Content: strings.Repeat("a", api.MaxContentLength+1),
	})
	require.Error(t, err)
	require.Equal(t, common.Invalid, common.ErrorCode(err))
}

func TestCreateMemo_DefaultVisibilityIsPrivate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	user := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, user.ID, &api.MemoCreate{
		Content: "hello world",
		// Visibility intentionally omitted – should default to Private.
	})
	require.NoError(t, err)
	require.Equal(t, api.Private, memo.Visibility)
}

func TestGetMemo_PrivateAccessAllowedForOwner(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "private memo",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	got, err := svc.GetMemo(ctx, &owner.ID, memo.ID)
	require.NoError(t, err)
	require.Equal(t, memo.ID, got.ID)
}

func TestGetMemo_PrivateAccessDeniedToStranger(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "private memo",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	strangerID := owner.ID + 999
	_, err = svc.GetMemo(ctx, &strangerID, memo.ID)
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestGetMemo_ProtectedAccessDeniedToAnonymous(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "protected memo",
		Visibility: api.Protected,
	})
	require.NoError(t, err)

	_, err = svc.GetMemo(ctx, nil, memo.ID)
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestDeleteMemo_OwnerCanDelete(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "my memo",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	err = svc.DeleteMemo(ctx, owner.ID, memo.ID)
	require.NoError(t, err)
}

func TestDeleteMemo_NonOwnerDenied(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "my memo",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	notOwnerID := owner.ID + 999
	err = svc.DeleteMemo(ctx, notOwnerID, memo.ID)
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestUpdateMemo_NonOwnerDenied(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	owner := createTestHostUser(ctx, svc, t)

	memo, err := svc.CreateMemo(ctx, owner.ID, &api.MemoCreate{
		Content:    "original content",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	newContent := "updated content"
	notOwnerID := owner.ID + 999
	_, err = svc.UpdateMemo(ctx, notOwnerID, memo.ID, &api.MemoPatch{Content: &newContent})
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestCreateMemo_DeniesResourceOwnedByOtherUser(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	memoOwner := createTestHostUser(ctx, svc, t)
	resourceOwner := createTestUser(ctx, svc, t, "resource-owner", api.NormalUser)
	resource, err := svc.CreateResource(ctx, resourceOwner.ID, &api.ResourceCreate{
		Filename: "secret.txt",
		Type:     "text/plain",
	})
	require.NoError(t, err)

	_, err = svc.CreateMemo(ctx, memoOwner.ID, &api.MemoCreate{
		Content:        "memo with foreign resource",
		ResourceIDList: []int{resource.ID},
	})
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestUpdateMemo_DeniesResourceOwnedByOtherUser(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	memoOwner := createTestHostUser(ctx, svc, t)
	resourceOwner := createTestUser(ctx, svc, t, "resource-owner", api.NormalUser)
	memo, err := svc.CreateMemo(ctx, memoOwner.ID, &api.MemoCreate{Content: "memo"})
	require.NoError(t, err)
	resource, err := svc.CreateResource(ctx, resourceOwner.ID, &api.ResourceCreate{
		Filename: "secret.txt",
		Type:     "text/plain",
	})
	require.NoError(t, err)

	_, err = svc.UpdateMemo(ctx, memoOwner.ID, memo.ID, &api.MemoPatch{
		ResourceIDList: []int{resource.ID},
	})
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}
