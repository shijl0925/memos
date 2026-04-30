package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	storedb "github.com/usememos/memos/store/db"
	"github.com/usememos/memos/test"
	"golang.org/x/crypto/bcrypt"
)

func TestStoreMemoUserResourceRelations(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(ctx, t)

	host := createStoreTestUser(ctx, t, s, "host", api.Host)
	user := createStoreTestUser(ctx, t, s, "user", api.NormalUser)

	firstCreatedTs := int64(100)
	firstMemo, err := s.CreateMemo(ctx, &api.MemoCreate{
		CreatorID:  host.ID,
		CreatedTs:  &firstCreatedTs,
		Content:    "#tag hello https://example.com\n- [ ] task\n`code`",
		Visibility: api.Public,
	})
	require.NoError(t, err)
	require.Equal(t, host.Nickname, firstMemo.CreatorName)

	secondCreatedTs := int64(200)
	secondMemo, err := s.CreateMemo(ctx, &api.MemoCreate{
		CreatorID:  user.ID,
		CreatedTs:  &secondCreatedTs,
		Content:    "private memo",
		Visibility: api.Private,
	})
	require.NoError(t, err)

	patchedContent := "updated public memo https://example.com\n- [ ] task\n`code`"
	patchedVisibility := api.Protected
	patchedCreatedTs := int64(150)
	patchedMemo, err := s.PatchMemo(ctx, &api.MemoPatch{
		ID:         firstMemo.ID,
		CreatedTs:  &patchedCreatedTs,
		Content:    &patchedContent,
		Visibility: &patchedVisibility,
	})
	require.NoError(t, err)
	require.Equal(t, patchedContent, patchedMemo.Content)
	require.Equal(t, patchedVisibility, patchedMemo.Visibility)
	require.Equal(t, patchedCreatedTs, patchedMemo.DisplayTs)

	firstResource, err := s.CreateResource(ctx, &api.ResourceCreate{
		CreatorID: host.ID,
		Filename:  "first.png",
		Blob:      []byte("first"),
		Type:      "image/png",
		Size:      5,
	})
	require.NoError(t, err)
	secondResource, err := s.CreateResource(ctx, &api.ResourceCreate{
		CreatorID:    host.ID,
		Filename:     "second.txt",
		InternalPath: "/tmp/second.txt",
		Type:         "text/plain",
		Size:         6,
	})
	require.NoError(t, err)

	_, err = s.UpsertMemoResource(ctx, &api.MemoResourceUpsert{MemoID: firstMemo.ID, ResourceID: secondResource.ID})
	require.NoError(t, err)
	_, err = s.UpsertMemoResource(ctx, &api.MemoResourceUpsert{MemoID: firstMemo.ID, ResourceID: firstResource.ID})
	require.NoError(t, err)
	_, err = s.UpsertMemoResource(ctx, &api.MemoResourceUpsert{MemoID: secondMemo.ID, ResourceID: firstResource.ID})
	require.NoError(t, err)
	memoResources, err := s.FindMemoResourceList(ctx, &api.MemoResourceFind{MemoID: &firstMemo.ID})
	require.NoError(t, err)
	require.Len(t, memoResources, 2)
	foundMemoResource, err := s.FindMemoResource(ctx, &api.MemoResourceFind{MemoID: &firstMemo.ID, ResourceID: &firstResource.ID})
	require.NoError(t, err)
	require.Equal(t, firstResource.ID, foundMemoResource.ResourceID)

	patchedFilename := "renamed-first.png"
	firstResource, err = s.PatchResource(ctx, &api.ResourcePatch{ID: firstResource.ID, Filename: &patchedFilename})
	require.NoError(t, err)
	require.Equal(t, patchedFilename, firstResource.Filename)
	foundResource, err := s.FindResource(ctx, &api.ResourceFind{Filename: &patchedFilename})
	require.NoError(t, err)
	require.Equal(t, firstResource.ID, foundResource.ID)

	relation, err := s.UpsertMemoRelation(ctx, &api.MemoRelation{
		MemoID:        firstMemo.ID,
		RelatedMemoID: secondMemo.ID,
		Type:          api.MemoRelationReference,
	})
	require.NoError(t, err)
	require.Equal(t, firstMemo.ID, relation.MemoID)

	err = s.UpsertMemoOrganizer(ctx, &api.MemoOrganizerUpsert{
		MemoID: firstMemo.ID,
		UserID: host.ID,
		Pinned: true,
	})
	require.NoError(t, err)

	foundOrganizer, err := s.FindMemoOrganizer(ctx, &api.MemoOrganizerFind{MemoID: firstMemo.ID, UserID: host.ID})
	require.NoError(t, err)
	require.True(t, foundOrganizer.Pinned)

	foundMemo, err := s.FindMemo(ctx, &api.MemoFind{ID: &firstMemo.ID})
	require.NoError(t, err)
	require.Len(t, foundMemo.ResourceList, 2)
	require.Len(t, foundMemo.RelationList, 1)
	require.Equal(t, secondMemo.ID, foundMemo.RelationList[0].RelatedMemoID)
	require.Equal(t, 2, foundMemo.ResourceList[0].LinkedMemoAmount)

	pinned := true
	hasLink := true
	hasTaskList := true
	hasCode := true
	createdAfter := int64(149)
	createdBefore := int64(151)
	memoList, err := s.FindMemoList(ctx, &api.MemoFind{
		CreatorID:           &host.ID,
		Pinned:              &pinned,
		ContentContainsList: []string{"updated"},
		VisibilityList:      []api.Visibility{api.Protected},
		HasLink:             &hasLink,
		HasTaskList:         &hasTaskList,
		HasCode:             &hasCode,
		CreatedTsAfter:      &createdAfter,
		CreatedTsBefore:     &createdBefore,
	})
	require.NoError(t, err)
	require.Len(t, memoList, 1)
	require.Equal(t, firstMemo.ID, memoList[0].ID)

	err = s.DeleteMemoRelation(ctx, &api.MemoRelationDelete{MemoID: &firstMemo.ID, RelatedMemoID: &secondMemo.ID})
	require.NoError(t, err)
	relations, err := s.FindMemoRelationList(ctx, &api.MemoRelationFind{MemoID: &firstMemo.ID})
	require.NoError(t, err)
	require.Empty(t, relations)

	err = s.DeleteMemoOrganizer(ctx, &api.MemoOrganizerDelete{MemoID: &firstMemo.ID, UserID: &host.ID})
	require.NoError(t, err)

	err = s.DeleteMemoResource(ctx, &api.MemoResourceDelete{MemoID: &firstMemo.ID, ResourceID: &secondResource.ID})
	require.NoError(t, err)

	err = s.DeleteMemo(ctx, &api.MemoDelete{ID: secondMemo.ID})
	require.NoError(t, err)
	err = s.DeleteResource(ctx, &api.ResourceDelete{ID: secondResource.ID})
	require.NoError(t, err)
}

func TestStoreShortcutTagSettingsStorageAndIdentityProvider(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(ctx, t)
	user := createStoreTestUser(ctx, t, s, "settings", api.Host)

	newNickname := "settings_updated"
	newEmail := "settings@example.org"
	user, err := s.PatchUser(ctx, &api.UserPatch{
		ID:       user.ID,
		Nickname: &newNickname,
		Email:    &newEmail,
	})
	require.NoError(t, err)
	require.Equal(t, newNickname, user.Nickname)
	require.Equal(t, newEmail, user.Email)
	foundUser, err := s.FindUser(ctx, &api.UserFind{ID: &user.ID})
	require.NoError(t, err)
	require.Equal(t, user.Username, foundUser.Username)
	users, err := s.FindUserList(ctx, &api.UserFind{})
	require.NoError(t, err)
	require.Len(t, users, 1)

	systemSetting, err := s.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:        api.SystemSettingAllowSignUpName,
		Value:       "true",
		Description: "allow test signups",
	})
	require.NoError(t, err)
	require.Equal(t, api.SystemSettingAllowSignUpName, systemSetting.Name)
	foundSystemSetting, err := s.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingAllowSignUpName})
	require.NoError(t, err)
	require.Equal(t, "true", foundSystemSetting.Value)
	systemSettings, err := s.FindSystemSettingList(ctx, &api.SystemSettingFind{})
	require.NoError(t, err)
	require.Len(t, systemSettings, 1)

	s.profile.Mode = "dev"
	activity, err := s.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityUserUpdate,
		Level:     api.ActivityInfo,
		Payload:   `{"field":"nickname"}`,
	})
	require.NoError(t, err)
	require.Equal(t, api.ActivityUserUpdate, activity.Type)
	s.profile.Mode = "prod"

	tag, err := s.UpsertTag(ctx, &api.TagUpsert{Name: "project", CreatorID: user.ID})
	require.NoError(t, err)
	require.Equal(t, "project", tag.Name)
	tagList, err := s.FindTagList(ctx, &api.TagFind{CreatorID: user.ID})
	require.NoError(t, err)
	require.Len(t, tagList, 1)
	err = s.DeleteTag(ctx, &api.TagDelete{Name: "project", CreatorID: user.ID})
	require.NoError(t, err)

	shortcut, err := s.CreateShortcut(ctx, &api.ShortcutCreate{
		CreatorID: user.ID,
		Title:     "Mine",
		Payload:   `{"visibility":"PUBLIC"}`,
	})
	require.NoError(t, err)
	patchedTitle := "Updated"
	patchedPayload := `{"visibility":"PRIVATE"}`
	shortcut, err = s.PatchShortcut(ctx, &api.ShortcutPatch{
		ID:      shortcut.ID,
		Title:   &patchedTitle,
		Payload: &patchedPayload,
	})
	require.NoError(t, err)
	require.Equal(t, patchedTitle, shortcut.Title)
	foundShortcut, err := s.FindShortcut(ctx, &api.ShortcutFind{ID: &shortcut.ID})
	require.NoError(t, err)
	require.Equal(t, patchedPayload, foundShortcut.Payload)
	shortcutList, err := s.FindShortcutList(ctx, &api.ShortcutFind{CreatorID: &user.ID})
	require.NoError(t, err)
	require.Len(t, shortcutList, 1)
	err = s.DeleteShortcut(ctx, &api.ShortcutDelete{ID: &shortcut.ID})
	require.NoError(t, err)

	locale, err := s.UpsertUserSetting(ctx, &api.UserSettingUpsert{
		UserID: user.ID,
		Key:    api.UserSettingLocaleKey,
		Value:  `"en"`,
	})
	require.NoError(t, err)
	require.Equal(t, `"en"`, locale.Value)
	setting, err := s.FindUserSetting(ctx, &api.UserSettingFind{UserID: user.ID, Key: api.UserSettingLocaleKey})
	require.NoError(t, err)
	require.Equal(t, api.UserSettingLocaleKey, setting.Key)
	settings, err := s.FindUserSettingList(ctx, &api.UserSettingFind{UserID: user.ID})
	require.NoError(t, err)
	require.Len(t, settings, 1)

	storageConfig := &api.StorageConfig{S3Config: &api.StorageS3Config{
		EndPoint: "https://s3.example.com",
		Region:   "us-east-1",
		Bucket:   "memos",
	}}
	storage, err := s.CreateStorage(ctx, &api.StorageCreate{
		Name:   "s3",
		Type:   api.StorageS3,
		Config: storageConfig,
	})
	require.NoError(t, err)
	newStorageName := "s3-updated"
	storage, err = s.PatchStorage(ctx, &api.StoragePatch{
		ID:     storage.ID,
		Type:   api.StorageS3,
		Name:   &newStorageName,
		Config: storageConfig,
	})
	require.NoError(t, err)
	require.Equal(t, newStorageName, storage.Name)
	foundStorage, err := s.FindStorage(ctx, &api.StorageFind{ID: &storage.ID})
	require.NoError(t, err)
	require.Equal(t, "memos", foundStorage.Config.S3Config.Bucket)
	storageList, err := s.FindStorageList(ctx, &api.StorageFind{})
	require.NoError(t, err)
	require.Len(t, storageList, 1)
	err = s.DeleteStorage(ctx, &api.StorageDelete{ID: storage.ID})
	require.NoError(t, err)

	idpConfig := &IdentityProviderConfig{OAuth2Config: &IdentityProviderOAuth2Config{
		ClientID:     "client",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		Scopes:       []string{"openid", "email"},
		FieldMapping: &FieldMapping{Identifier: "sub", DisplayName: "name", Email: "email"},
	}}
	idp, err := s.CreateIdentityProvider(ctx, &IdentityProviderMessage{
		Name:             "oauth",
		Type:             IdentityProviderOAuth2,
		IdentifierFilter: "example.com",
		Config:           idpConfig,
	})
	require.NoError(t, err)
	newIDPName := "oauth-updated"
	idp, err = s.UpdateIdentityProvider(ctx, &UpdateIdentityProviderMessage{
		ID:     idp.ID,
		Type:   IdentityProviderOAuth2,
		Name:   &newIDPName,
		Config: idpConfig,
	})
	require.NoError(t, err)
	require.Equal(t, newIDPName, idp.Name)
	foundIDP, err := s.GetIdentityProvider(ctx, &FindIdentityProviderMessage{ID: &idp.ID})
	require.NoError(t, err)
	require.Equal(t, "client", foundIDP.Config.OAuth2Config.ClientID)
	idpList, err := s.ListIdentityProviders(ctx, &FindIdentityProviderMessage{})
	require.NoError(t, err)
	require.Len(t, idpList, 1)
	err = s.DeleteIdentityProvider(ctx, &DeleteIdentityProviderMessage{ID: idp.ID})
	require.NoError(t, err)

	err = s.DeleteUser(ctx, &api.UserDelete{ID: user.ID})
	require.NoError(t, err)
	users, err = s.FindUserList(ctx, &api.UserFind{})
	require.NoError(t, err)
	require.Empty(t, users)
	require.NoError(t, s.Vacuum(ctx))
}

func TestStoreMySQLCompatibleBranches(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(ctx, t)
	s.driver = "mysql"
	s.profile.Driver = "mysql"

	user := createStoreTestUser(ctx, t, s, "mysqluser", api.Host)
	mysqlNickname := "mysql_nickname"
	user, err := s.PatchUser(ctx, &api.UserPatch{ID: user.ID, Nickname: &mysqlNickname})
	require.NoError(t, err)
	require.Equal(t, mysqlNickname, user.Nickname)
	users, err := s.FindUserList(ctx, &api.UserFind{Nickname: &mysqlNickname})
	require.NoError(t, err)
	require.Len(t, users, 1)

	memo, err := s.CreateMemo(ctx, &api.MemoCreate{
		CreatorID:  user.ID,
		Content:    "mysql branch memo",
		Visibility: api.Public,
	})
	require.NoError(t, err)
	mysqlContent := "mysql branch memo updated"
	memo, err = s.PatchMemo(ctx, &api.MemoPatch{ID: memo.ID, Content: &mysqlContent})
	require.NoError(t, err)
	require.Equal(t, mysqlContent, memo.Content)

	resource, err := s.CreateResource(ctx, &api.ResourceCreate{
		CreatorID: user.ID,
		Filename:  "mysql.png",
		Blob:      []byte("mysql"),
		Type:      "image/png",
		Size:      5,
	})
	require.NoError(t, err)
	mysqlFilename := "mysql-renamed.png"
	resource, err = s.PatchResource(ctx, &api.ResourcePatch{ID: resource.ID, Filename: &mysqlFilename})
	require.NoError(t, err)
	require.Equal(t, mysqlFilename, resource.Filename)

	shortcut, err := s.CreateShortcut(ctx, &api.ShortcutCreate{
		CreatorID: user.ID,
		Title:     "mysql shortcut",
		Payload:   "{}",
	})
	require.NoError(t, err)
	mysqlShortcutTitle := "mysql shortcut updated"
	shortcut, err = s.PatchShortcut(ctx, &api.ShortcutPatch{ID: shortcut.ID, Title: &mysqlShortcutTitle})
	require.NoError(t, err)
	require.Equal(t, mysqlShortcutTitle, shortcut.Title)

	storageConfig := &api.StorageConfig{S3Config: &api.StorageS3Config{Bucket: "mysql-bucket"}}
	storage, err := s.CreateStorage(ctx, &api.StorageCreate{
		Name:   "mysql storage",
		Type:   api.StorageS3,
		Config: storageConfig,
	})
	require.NoError(t, err)
	mysqlStorageName := "mysql storage updated"
	storage, err = s.PatchStorage(ctx, &api.StoragePatch{
		ID:     storage.ID,
		Type:   api.StorageS3,
		Name:   &mysqlStorageName,
		Config: storageConfig,
	})
	require.NoError(t, err)
	require.Equal(t, mysqlStorageName, storage.Name)

	idpConfig := &IdentityProviderConfig{OAuth2Config: &IdentityProviderOAuth2Config{
		ClientID:     "mysql-client",
		ClientSecret: "mysql-secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		FieldMapping: &FieldMapping{Identifier: "sub"},
	}}
	idp, err := s.CreateIdentityProvider(ctx, &IdentityProviderMessage{
		Name:   "mysql idp",
		Type:   IdentityProviderOAuth2,
		Config: idpConfig,
	})
	require.NoError(t, err)
	mysqlIDPName := "mysql idp updated"
	idp, err = s.UpdateIdentityProvider(ctx, &UpdateIdentityProviderMessage{
		ID:   idp.ID,
		Type: IdentityProviderOAuth2,
		Name: &mysqlIDPName,
	})
	require.NoError(t, err)
	require.Equal(t, mysqlIDPName, idp.Name)

	s.profile.Mode = "dev"
	activity, err := s.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: user.ID,
		Type:      api.ActivityMemoCreate,
		Level:     api.ActivityInfo,
		Payload:   "{}",
	})
	require.NoError(t, err)
	require.Equal(t, api.ActivityMemoCreate, activity.Type)

	require.NoError(t, s.DeleteIdentityProvider(ctx, &DeleteIdentityProviderMessage{ID: idp.ID}))
	require.NoError(t, s.DeleteStorage(ctx, &api.StorageDelete{ID: storage.ID}))
	require.NoError(t, s.DeleteShortcut(ctx, &api.ShortcutDelete{ID: &shortcut.ID}))
	require.NoError(t, s.DeleteResource(ctx, &api.ResourceDelete{ID: resource.ID}))
	require.NoError(t, s.DeleteMemo(ctx, &api.MemoDelete{ID: memo.ID}))
	require.NoError(t, s.DeleteUser(ctx, &api.UserDelete{ID: user.ID}))
}

func newTestingStore(ctx context.Context, t *testing.T) *Store {
	t.Helper()

	profile := test.GetTestingProfile(t)
	database := storedb.NewDB(profile)
	require.NoError(t, database.Open(ctx))
	t.Cleanup(func() {
		require.NoError(t, database.DBInstance.Close())
	})
	return New(database.DBInstance, profile)
}

func createStoreTestUser(ctx context.Context, t *testing.T, s *Store, username string, role api.Role) *api.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(username+"_password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	user, err := s.CreateUser(ctx, &api.UserCreate{
		Username:     username,
		Role:         role,
		Email:        username + "@example.com",
		Nickname:     username + "_nickname",
		Password:     username + "_password",
		PasswordHash: string(passwordHash),
		OpenID:       username + "_openid",
	})
	require.NoError(t, err)
	return user
}
