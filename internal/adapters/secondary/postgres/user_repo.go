package pgadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/user"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (user.User, error) {
	return r.getUser(ctx, `WHERE u.id = $1`, id)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return r.getUser(ctx, `WHERE u.email = $1`, email)
}

func (r *UserRepo) Create(ctx context.Context, u user.User) (user.User, error) {
	u.ID = uuid.NewString()
	if u.Role == "" {
		u.Role = "user"
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.PasswordHash, u.Role, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return user.User{}, fmt.Errorf("%w: email %q already registered", domain.ErrAlreadyExists, u.Email)
		}
		return user.User{}, fmt.Errorf("create user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, excluded_allergens, dietary_restrictions, updated_at)
		VALUES ($1, '[]', '[]', $2)`,
		u.ID, now,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("create user preferences: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return user.User{}, err
	}
	return u, nil
}

func (r *UserRepo) UpdatePreferences(ctx context.Context, id string, prefs user.Preferences) error {
	allergensJSON, err := toJSON(prefs.ExcludedAllergens)
	if err != nil {
		return err
	}
	restrictionsJSON, err := toJSON(prefs.DietaryRestrictions)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE user_preferences
		SET excluded_allergens = $1, dietary_restrictions = $2, updated_at = $3
		WHERE user_id = $4`,
		allergensJSON, restrictionsJSON, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update preferences: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (r *UserRepo) getUser(ctx context.Context, where string, arg any) (user.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.password_hash, u.role, u.created_at, u.updated_at,
		       COALESCE(p.excluded_allergens, '[]'),
		       COALESCE(p.dietary_restrictions, '[]')
		FROM users u
		LEFT JOIN user_preferences p ON p.user_id = u.id
		`+where, arg)

	var (
		id, email, passwordHash, role string
		createdAt, updatedAt          time.Time
		allergensJSON, dietsJSON      string
	)
	if err := row.Scan(
		&id, &email, &passwordHash, &role, &createdAt, &updatedAt,
		&allergensJSON, &dietsJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, domain.ErrNotFound
		}
		return user.User{}, fmt.Errorf("get user: %w", err)
	}

	var allergens, diets []string
	if err := fromJSON(allergensJSON, &allergens); err != nil {
		return user.User{}, err
	}
	if err := fromJSON(dietsJSON, &diets); err != nil {
		return user.User{}, err
	}

	return user.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Preferences: user.Preferences{
			ExcludedAllergens:   allergens,
			DietaryRestrictions: diets,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
