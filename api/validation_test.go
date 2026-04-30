package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnumStringMethods(t *testing.T) {
	require.Equal(t, "NORMAL", Normal.String())
	require.Equal(t, "ARCHIVED", Archived.String())
	require.Empty(t, RowStatus("unknown").String())

	require.Equal(t, "HOST", Host.String())
	require.Equal(t, "ADMIN", Admin.String())
	require.Equal(t, "USER", NormalUser.String())
	require.Equal(t, "USER", Role("unknown").String())

	require.Equal(t, "PUBLIC", Public.String())
	require.Equal(t, "PROTECTED", Protected.String())
	require.Equal(t, "PRIVATE", Private.String())
	require.Equal(t, "PRIVATE", Visibility("unknown").String())

	require.Equal(t, "allow-signup", SystemSettingAllowSignUpName.String())
	require.Equal(t, "openai-config", SystemSettingOpenAIConfigName.String())
	require.Empty(t, SystemSettingName("unknown").String())

	require.Equal(t, "locale", UserSettingLocaleKey.String())
	require.Equal(t, "appearance", UserSettingAppearanceKey.String())
	require.Equal(t, "memo-visibility", UserSettingMemoVisibilityKey.String())
	require.Empty(t, UserSettingKey("unknown").String())
}

func TestUserCreateValidate(t *testing.T) {
	valid := UserCreate{
		Username: "alice",
		Password: "secret",
		Nickname: "Alice",
		Email:    "alice@example.com",
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		mutate func(*UserCreate)
	}{
		{name: "short username", mutate: func(create *UserCreate) { create.Username = "ab" }},
		{name: "long username", mutate: func(create *UserCreate) { create.Username = strings.Repeat("a", 33) }},
		{name: "short password", mutate: func(create *UserCreate) { create.Password = "ab" }},
		{name: "long password", mutate: func(create *UserCreate) { create.Password = strings.Repeat("a", 513) }},
		{name: "long nickname", mutate: func(create *UserCreate) { create.Nickname = strings.Repeat("a", 65) }},
		{name: "long email", mutate: func(create *UserCreate) { create.Email = strings.Repeat("a", 250) + "@example.com" }},
		{name: "invalid email", mutate: func(create *UserCreate) { create.Email = "@example.com" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			create := valid
			test.mutate(&create)
			require.Error(t, create.Validate())
		})
	}
}

func TestUserPatchValidate(t *testing.T) {
	username := "alice"
	password := "secret"
	nickname := "Alice"
	email := "alice@example.com"
	avatar := "https://example.com/avatar.png"
	require.NoError(t, UserPatch{
		Username:  &username,
		Password:  &password,
		Nickname:  &nickname,
		Email:     &email,
		AvatarURL: &avatar,
	}.Validate())

	tests := []struct {
		name  string
		patch UserPatch
	}{
		{name: "short username", patch: UserPatch{Username: stringPtr("ab")}},
		{name: "long username", patch: UserPatch{Username: stringPtr(strings.Repeat("a", 33))}},
		{name: "short password", patch: UserPatch{Password: stringPtr("ab")}},
		{name: "long password", patch: UserPatch{Password: stringPtr(strings.Repeat("a", 513))}},
		{name: "long nickname", patch: UserPatch{Nickname: stringPtr(strings.Repeat("a", 65))}},
		{name: "large avatar", patch: UserPatch{AvatarURL: stringPtr(strings.Repeat("a", 2<<20+1))}},
		{name: "long email", patch: UserPatch{Email: stringPtr(strings.Repeat("a", 250) + "@example.com")}},
		{name: "invalid email", patch: UserPatch{Email: stringPtr("@example.com")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, test.patch.Validate())
		})
	}
}

func TestSystemSettingValidate(t *testing.T) {
	validSettings := []SystemSettingUpsert{
		{Name: SystemSettingAllowSignUpName, Value: "true"},
		{Name: SystemSettingIgnoreUpgradeName, Value: "false"},
		{Name: SystemSettingDisablePublicMemosName, Value: "true"},
		{Name: SystemSettingAdditionalStyleName, Value: `"body { color: red; }"`},
		{Name: SystemSettingAdditionalScriptName, Value: `"console.log('ok')"`},
		{Name: SystemSettingCustomizedProfileName, Value: `{"name":"memos","locale":"en","appearance":"system"}`},
		{Name: SystemSettingStorageServiceIDName, Value: "0"},
		{Name: SystemSettingLocalStoragePathName, Value: `"/tmp/memos"`},
		{Name: SystemSettingOpenAIConfigName, Value: `{"key":"test","host":"https://example.com"}`},
	}
	for _, setting := range validSettings {
		require.NoError(t, setting.Validate(), setting.Name)
	}

	invalidSettings := []SystemSettingUpsert{
		{Name: SystemSettingServerIDName, Value: `"server"`},
		{Name: SystemSettingAllowSignUpName, Value: `"not-bool"`},
		{Name: SystemSettingIgnoreUpgradeName, Value: `"not-bool"`},
		{Name: SystemSettingDisablePublicMemosName, Value: `"not-bool"`},
		{Name: SystemSettingAdditionalStyleName, Value: "true"},
		{Name: SystemSettingAdditionalScriptName, Value: "true"},
		{Name: SystemSettingCustomizedProfileName, Value: `{"locale":"invalid","appearance":"system"}`},
		{Name: SystemSettingCustomizedProfileName, Value: `{"locale":"en","appearance":"invalid"}`},
		{Name: SystemSettingStorageServiceIDName, Value: `"not-number"`},
		{Name: SystemSettingLocalStoragePathName, Value: "true"},
		{Name: SystemSettingOpenAIConfigName, Value: `"not-object"`},
		{Name: SystemSettingName("unknown"), Value: "true"},
	}
	for _, setting := range invalidSettings {
		require.Error(t, setting.Validate(), setting.Name)
	}
}

func TestUserSettingValidate(t *testing.T) {
	require.NoError(t, UserSettingUpsert{Key: UserSettingLocaleKey, Value: `"en"`}.Validate())
	require.NoError(t, UserSettingUpsert{Key: UserSettingAppearanceKey, Value: `"dark"`}.Validate())
	require.NoError(t, UserSettingUpsert{Key: UserSettingMemoVisibilityKey, Value: `"PUBLIC"`}.Validate())

	invalidSettings := []UserSettingUpsert{
		{Key: UserSettingLocaleKey, Value: "true"},
		{Key: UserSettingLocaleKey, Value: `"invalid"`},
		{Key: UserSettingAppearanceKey, Value: "true"},
		{Key: UserSettingAppearanceKey, Value: `"invalid"`},
		{Key: UserSettingMemoVisibilityKey, Value: "true"},
		{Key: UserSettingMemoVisibilityKey, Value: `"INVALID"`},
		{Key: UserSettingKey("unknown"), Value: `"en"`},
	}
	for _, setting := range invalidSettings {
		require.Error(t, setting.Validate(), setting.Key)
	}
}

func stringPtr(value string) *string {
	return &value
}
