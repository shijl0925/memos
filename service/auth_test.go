package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func TestSignIn_CorrectCredentials(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	_, err := svc.SignUp(ctx, &api.SignUp{
		Username: "testuser",
		Password: "testpassword",
	}, "127.0.0.1")
	require.NoError(t, err)

	got, err := svc.SignIn(ctx, "testuser", "testpassword")
	require.NoError(t, err)
	require.Equal(t, "testuser", got.Username)
}

func TestSignIn_WrongPassword(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	_, err := svc.SignUp(ctx, &api.SignUp{
		Username: "testuser",
		Password: "testpassword",
	}, "127.0.0.1")
	require.NoError(t, err)

	_, err = svc.SignIn(ctx, "testuser", "wrongpassword")
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestSignIn_UnknownUser(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	_, err := svc.SignIn(ctx, "nobody", "password")
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestSignUp_FirstUserBecomesHost(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	user, err := svc.SignUp(ctx, &api.SignUp{
		Username: "firstuser",
		Password: "password123",
	}, "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, api.Host, user.Role)
}

func TestSignUp_SubsequentUserIsNormalUser(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	// First user (host)
	_, err := svc.SignUp(ctx, &api.SignUp{
		Username: "firstuser",
		Password: "password123",
	}, "127.0.0.1")
	require.NoError(t, err)

	// Enable sign-up via system setting
	_, err = svc.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:  api.SystemSettingAllowSignUpName,
		Value: "true",
	})
	require.NoError(t, err)

	user, err := svc.SignUp(ctx, &api.SignUp{
		Username: "seconduser",
		Password: "password456",
	}, "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, api.NormalUser, user.Role)
}

func TestSignUp_SignUpDisabledByDefault(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	// First user (host)
	_, err := svc.SignUp(ctx, &api.SignUp{
		Username: "firstuser",
		Password: "password123",
	}, "127.0.0.1")
	require.NoError(t, err)

	// No allowSignUp setting → signup disabled by default
	_, err = svc.SignUp(ctx, &api.SignUp{
		Username: "seconduser",
		Password: "password456",
	}, "127.0.0.1")
	require.Error(t, err)
	require.Equal(t, common.NotAuthorized, common.ErrorCode(err))
}

func TestSignUp_PasswordTooShort(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(ctx, t)

	_, err := svc.SignUp(ctx, &api.SignUp{
		Username: "user",
		Password: "ab", // too short
	}, "127.0.0.1")
	require.Error(t, err)
	require.Equal(t, common.Invalid, common.ErrorCode(err))
}
