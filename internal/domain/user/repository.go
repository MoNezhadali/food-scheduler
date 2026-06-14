package user

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u User) (User, error)
	UpdatePreferences(ctx context.Context, id string, prefs Preferences) error
}
