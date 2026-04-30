package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestGenerateTokens(t *testing.T) {
	tests := []struct {
		name      string
		generate  func(string, int, string) (string, error)
		audience  string
		expiresIn time.Duration
	}{
		{
			name:      "api token",
			generate:  GenerateAPIToken,
			audience:  AccessTokenAudienceName,
			expiresIn: apiTokenDuration,
		},
		{
			name:      "access token",
			generate:  GenerateAccessToken,
			audience:  AccessTokenAudienceName,
			expiresIn: accessTokenDuration,
		},
		{
			name:      "refresh token",
			generate:  GenerateRefreshToken,
			audience:  RefreshTokenAudienceName,
			expiresIn: refreshTokenDuration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issuedBefore := time.Now()
			tokenString, err := test.generate("alice", 101, "test-secret")
			require.NoError(t, err)
			require.NotEmpty(t, tokenString)

			claims := parseAndValidateToken(t, tokenString, "test-secret")
			require.Equal(t, "alice", claims.Name)
			require.Equal(t, issuer, claims.Issuer)
			require.Equal(t, "101", claims.Subject)
			require.True(t, claims.VerifyAudience(test.audience, true))
			require.WithinDuration(t, issuedBefore.Add(test.expiresIn), claims.ExpiresAt.Time, 2*time.Second)
			require.GreaterOrEqual(t, claims.IssuedAt.Time.Unix(), issuedBefore.Add(-time.Second).Unix())
		})
	}
}

func TestTokenRejectsWrongSecret(t *testing.T) {
	tokenString, err := GenerateAccessToken("alice", 101, "right-secret")
	require.NoError(t, err)

	_, err = jwt.ParseWithClaims(tokenString, &claimsMessage{}, func(token *jwt.Token) (any, error) {
		return []byte("wrong-secret"), nil
	})
	require.Error(t, err)
}

func TestCookieExpirationIsShorterThanRefreshToken(t *testing.T) {
	require.Equal(t, refreshTokenDuration-time.Minute, CookieExpDuration)
	require.Greater(t, refreshTokenDuration, CookieExpDuration)
}

func parseAndValidateToken(t *testing.T, tokenString, secret string) *claimsMessage {
	t.Helper()

	token, err := jwt.ParseWithClaims(tokenString, &claimsMessage{}, func(token *jwt.Token) (any, error) {
		require.Equal(t, jwt.SigningMethodHS256.Alg(), token.Method.Alg())
		require.Equal(t, keyID, token.Header["kid"])
		return []byte(secret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims, ok := token.Claims.(*claimsMessage)
	require.True(t, ok)
	return claims
}
