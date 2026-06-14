package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	appuser "github.com/MoNezhadali/foodscheduler/internal/application/user"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type stubCreator struct {
	returnErr error
}

func (s *stubCreator) Create(_ context.Context, u domuser.User) (domuser.User, error) {
	if s.returnErr != nil {
		return domuser.User{}, s.returnErr
	}
	u.ID = "test-id"
	return u, nil
}

type stubEmailFinder struct {
	user      domuser.User
	returnErr error
}

func (s *stubEmailFinder) GetByEmail(_ context.Context, _ string) (domuser.User, error) {
	return s.user, s.returnErr
}

type stubIDFinder struct {
	user      domuser.User
	returnErr error
}

func (s *stubIDFinder) GetByID(_ context.Context, _ string) (domuser.User, error) {
	return s.user, s.returnErr
}

type stubTokenSvc struct {
	pair      auth.TokenPair
	claims    *auth.Claims
	returnErr error
}

func (s *stubTokenSvc) IssueTokens(_, _, _ string) (auth.TokenPair, error) {
	return s.pair, s.returnErr
}

func (s *stubTokenSvc) Validate(_ string, expectedType auth.TokenType) (*auth.Claims, error) {
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	c := *s.claims
	c.Type = expectedType
	return &c, nil
}

// ── RegisterUseCase ───────────────────────────────────────────────────────────

func TestRegister_OK(t *testing.T) {
	uc := appuser.NewRegisterUseCase(&stubCreator{})
	got, err := uc.Execute(context.Background(), appuser.RegisterCmd{
		Email:    "Alice@Example.COM",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", got.Email) // normalised
	assert.Equal(t, "user", got.Role)
	assert.NotEmpty(t, got.PasswordHash)
	assert.NotEqual(t, "password123", got.PasswordHash) // bcrypt hashed
}

func TestRegister_InvalidEmail(t *testing.T) {
	uc := appuser.NewRegisterUseCase(&stubCreator{})
	_, err := uc.Execute(context.Background(), appuser.RegisterCmd{
		Email: "notanemail", Password: "password123",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestRegister_ShortPassword(t *testing.T) {
	uc := appuser.NewRegisterUseCase(&stubCreator{})
	_, err := uc.Execute(context.Background(), appuser.RegisterCmd{
		Email: "alice@example.com", Password: "short",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	uc := appuser.NewRegisterUseCase(&stubCreator{returnErr: domain.ErrAlreadyExists})
	_, err := uc.Execute(context.Background(), appuser.RegisterCmd{
		Email: "alice@example.com", Password: "password123",
	})
	assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
}

// ── LoginUseCase ──────────────────────────────────────────────────────────────

func hashedPassword(t *testing.T, plain string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func TestLogin_OK(t *testing.T) {
	finder := &stubEmailFinder{user: domuser.User{
		ID:           "uid-1",
		Email:        "alice@example.com",
		Role:         "user",
		PasswordHash: hashedPassword(t, "password123"),
	}}
	svc := &stubTokenSvc{pair: auth.TokenPair{AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 900}}
	uc := appuser.NewLoginUseCase(finder, svc)

	pair, err := uc.Execute(context.Background(), "alice@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "acc", pair.AccessToken)
	assert.Equal(t, "ref", pair.RefreshToken)
}

func TestLogin_WrongPassword(t *testing.T) {
	finder := &stubEmailFinder{user: domuser.User{
		ID:           "uid-1",
		PasswordHash: hashedPassword(t, "password123"),
	}}
	uc := appuser.NewLoginUseCase(finder, &stubTokenSvc{})
	_, err := uc.Execute(context.Background(), "alice@example.com", "wrongpassword")
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestLogin_UserNotFound(t *testing.T) {
	finder := &stubEmailFinder{returnErr: domain.ErrNotFound}
	uc := appuser.NewLoginUseCase(finder, &stubTokenSvc{})
	_, err := uc.Execute(context.Background(), "nobody@example.com", "password123")
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

// ── RefreshUseCase ────────────────────────────────────────────────────────────

func TestRefresh_OK(t *testing.T) {
	finder := &stubIDFinder{user: domuser.User{
		ID: "uid-1", Email: "alice@example.com", Role: "user",
	}}
	svc := &stubTokenSvc{
		claims: &auth.Claims{UserID: "uid-1", Email: "alice@example.com", Role: "user"},
		pair:   auth.TokenPair{AccessToken: "new-acc", RefreshToken: "new-ref", ExpiresIn: 900},
	}
	uc := appuser.NewRefreshUseCase(finder, svc)

	pair, err := uc.Execute(context.Background(), "some-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-acc", pair.AccessToken)
}

func TestRefresh_InvalidToken(t *testing.T) {
	finder := &stubIDFinder{}
	svc := &stubTokenSvc{returnErr: errors.New("bad token")}
	uc := appuser.NewRefreshUseCase(finder, svc)
	_, err := uc.Execute(context.Background(), "bad-token")
	assert.Error(t, err)
}

func TestRefresh_UserDeleted(t *testing.T) {
	finder := &stubIDFinder{returnErr: domain.ErrNotFound}
	svc := &stubTokenSvc{
		claims: &auth.Claims{UserID: "uid-gone"},
	}
	uc := appuser.NewRefreshUseCase(finder, svc)
	_, err := uc.Execute(context.Background(), "some-refresh-token")
	assert.Error(t, err)
}
