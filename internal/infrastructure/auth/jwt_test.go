package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

func newSvc() *auth.JWTService {
	return auth.NewJWTService("test-secret-key")
}

func TestJWT_IssueAndValidateAccess(t *testing.T) {
	svc := newSvc()
	pair, err := svc.IssueTokens("uid-1", "alice@example.com", "user")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, 900, pair.ExpiresIn) // 15 min

	claims, err := svc.Validate(pair.AccessToken, auth.TokenTypeAccess)
	require.NoError(t, err)
	assert.Equal(t, "uid-1", claims.UserID)
	assert.Equal(t, "alice@example.com", claims.Email)
	assert.Equal(t, "user", claims.Role)
	assert.Equal(t, auth.TokenTypeAccess, claims.Type)
}

func TestJWT_ValidateRefreshToken(t *testing.T) {
	svc := newSvc()
	pair, err := svc.IssueTokens("uid-1", "alice@example.com", "admin")
	require.NoError(t, err)

	claims, err := svc.Validate(pair.RefreshToken, auth.TokenTypeRefresh)
	require.NoError(t, err)
	assert.Equal(t, "uid-1", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, auth.TokenTypeRefresh, claims.Type)
}

func TestJWT_WrongTokenType(t *testing.T) {
	svc := newSvc()
	pair, _ := svc.IssueTokens("uid-1", "alice@example.com", "user")

	// Access token validated as refresh → error
	_, err := svc.Validate(pair.AccessToken, auth.TokenTypeRefresh)
	assert.Error(t, err)
}

func TestJWT_InvalidSignature(t *testing.T) {
	svc1 := auth.NewJWTService("secret-one")
	svc2 := auth.NewJWTService("secret-two")

	pair, _ := svc1.IssueTokens("uid-1", "alice@example.com", "user")
	_, err := svc2.Validate(pair.AccessToken, auth.TokenTypeAccess)
	assert.Error(t, err)
}

func TestJWT_TamperedToken(t *testing.T) {
	svc := newSvc()
	pair, _ := svc.IssueTokens("uid-1", "alice@example.com", "user")

	_, err := svc.Validate(pair.AccessToken+"x", auth.TokenTypeAccess)
	assert.Error(t, err)
}
