package user

import (
	"context"
	"fmt"

	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

type userIDFinder interface {
	GetByID(ctx context.Context, id string) (domuser.User, error)
}

type RefreshUseCase struct {
	users    userIDFinder
	tokenSvc auth.Service
}

func NewRefreshUseCase(users userIDFinder, tokenSvc auth.Service) *RefreshUseCase {
	return &RefreshUseCase{users: users, tokenSvc: tokenSvc}
}

func (uc *RefreshUseCase) Execute(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	claims, err := uc.tokenSvc.Validate(refreshToken, auth.TokenTypeRefresh)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Confirm the user still exists (may have been deleted since token was issued)
	u, err := uc.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("user not found: %w", err)
	}

	tokens, err := uc.tokenSvc.IssueTokens(u.ID, u.Email, u.Role)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("issue tokens: %w", err)
	}
	return tokens, nil
}
