package sqliteadapter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/user"
)

func newTestUser() user.User {
	return user.User{
		Email:        "test@example.com",
		PasswordHash: "$2a$12$hashed",
	}
}

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestUser())
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "test@example.com", created.Email)
	assert.False(t, created.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "$2a$12$hashed", got.PasswordHash)
	assert.Empty(t, got.Preferences.ExcludedAllergens)
	assert.Empty(t, got.Preferences.DietaryRestrictions)
}

func TestUserRepo_GetByEmail(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestUser())
	require.NoError(t, err)

	got, err := repo.GetByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", got.Email)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	_, err := repo.GetByID(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_CreateDuplicateEmail(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestUser())
	require.NoError(t, err)

	_, err = repo.Create(ctx, newTestUser())
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestUserRepo_UpdatePreferences(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestUser())
	require.NoError(t, err)

	prefs := user.Preferences{
		ExcludedAllergens:   []string{"gluten", "dairy"},
		DietaryRestrictions: []string{"vegetarian"},
	}
	err = repo.UpdatePreferences(ctx, created.ID, prefs)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"gluten", "dairy"}, got.Preferences.ExcludedAllergens)
	assert.Equal(t, []string{"vegetarian"}, got.Preferences.DietaryRestrictions)
}

func TestUserRepo_UpdatePreferences_NotFound(t *testing.T) {
	repo := sqliteadapter.NewUserRepo(setupTestDB(t))
	err := repo.UpdatePreferences(context.Background(), "nonexistent", user.Preferences{})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
