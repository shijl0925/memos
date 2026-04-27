package service

import (
	"bytes"
	"context"
	"mime/multipart"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

type testMultipartFile struct {
	*bytes.Reader
}

func (f testMultipartFile) Close() error {
	return nil
}

func useLocalStorage(ctx context.Context, t *testing.T, svc *Service, localPath string) {
	t.Helper()
	_, err := svc.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:  api.SystemSettingStorageServiceIDName,
		Value: "-1",
	})
	require.NoError(t, err)
	_, err = svc.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:  api.SystemSettingLocalStoragePathName,
		Value: `"` + localPath + `"`,
	})
	require.NoError(t, err)
}

func TestCreateResourceFromBlob_SanitizesUploadedFilename(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	svc.Profile.Data = t.TempDir()
	user := createTestHostUser(ctx, svc, t)
	useLocalStorage(ctx, t, svc, "uploads/{filename}")

	resource, err := svc.CreateResourceFromBlob(ctx, user.ID, testMultipartFile{bytes.NewReader([]byte("hello"))}, &multipart.FileHeader{
		Filename: "../../evil.txt",
		Size:     int64(len("hello")),
	})
	require.NoError(t, err)
	require.Equal(t, "evil.txt", resource.Filename)
	require.True(t, strings.HasPrefix(resource.InternalPath, svc.Profile.Data+string(filepath.Separator)))
	require.FileExists(t, filepath.Join(svc.Profile.Data, "uploads", "evil.txt"))
}

func TestCreateResourceFromBlob_DeniesLocalStoragePathTraversal(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)
	svc.Profile.Data = t.TempDir()
	user := createTestHostUser(ctx, svc, t)
	useLocalStorage(ctx, t, svc, "../{filename}")

	_, err := svc.CreateResourceFromBlob(ctx, user.ID, testMultipartFile{bytes.NewReader([]byte("hello"))}, &multipart.FileHeader{
		Filename: "evil.txt",
		Size:     int64(len("hello")),
	})
	require.Error(t, err)
	require.Equal(t, common.Invalid, common.ErrorCode(err))
}
