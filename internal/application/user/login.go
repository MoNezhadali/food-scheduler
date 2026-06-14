package user

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

type userEmailFinder interface {
	GetByEmail(ctx context.Context, email string) (domuser.User, error)
}

type LoginUseCase struct {
	users    userEmailFinder
	tokenSvc auth.Service
}

func NewLoginUseCase(users userEmailFinder, tokenSvc auth.Service) *LoginUseCase {
	return &LoginUseCase{users: users, tokenSvc: tokenSvc}
}

func (uc *LoginUseCase) Execute(ctx context.Context, email, password string) (auth.TokenPair, error) {
	u, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists
		return auth.TokenPair{}, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return auth.TokenPair{}, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}

	tokens, err := uc.tokenSvc.IssueTokens(u.ID, u.Email, u.Role)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("issue tokens: %w", err)
	}
	return tokens, nil
}
