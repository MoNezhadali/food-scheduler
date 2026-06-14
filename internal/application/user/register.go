package user

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
)

type userCreator interface {
	Create(ctx context.Context, u domuser.User) (domuser.User, error)
}

type RegisterCmd struct {
	Email    string
	Password string
}

type RegisterUseCase struct {
	users userCreator
}

func NewRegisterUseCase(users userCreator) *RegisterUseCase {
	return &RegisterUseCase{users: users}
}

func (uc *RegisterUseCase) Execute(ctx context.Context, cmd RegisterCmd) (domuser.User, error) {
	cmd.Email = strings.TrimSpace(strings.ToLower(cmd.Email))
	if !strings.Contains(cmd.Email, "@") {
		return domuser.User{}, fmt.Errorf("%w: invalid email", domain.ErrInvalidInput)
	}
	if len(cmd.Password) < 8 {
		return domuser.User{}, fmt.Errorf("%w: password must be at least 8 characters", domain.ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return domuser.User{}, fmt.Errorf("hash password: %w", err)
	}

	return uc.users.Create(ctx, domuser.User{
		Email:        cmd.Email,
		PasswordHash: string(hash),
		Role:         "user",
	})
}
