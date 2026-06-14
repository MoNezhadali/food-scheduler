package sqliteadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
	return r.getUser(ctx, `WHERE u.id = ?`, id)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return r.getUser(ctx, `WHERE u.email = ?`, email)
}

func (r *UserRepo) Create(ctx context.Context, u user.User) (user.User, error) {
	u.ID = uuid.NewString()
	ts := nowStr()
	u.CreatedAt, _ = parseTime(ts)
	u.UpdatedAt = u.CreatedAt

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, ts, ts,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return user.User{}, fmt.Errorf("%w: email %q already registered", domain.ErrAlreadyExists, u.Email)
		}
		return user.User{}, fmt.Errorf("create user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, excluded_allergens, dietary_restrictions, updated_at)
		VALUES (?, '[]', '[]', ?)`,
		u.ID, ts,
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
		SET excluded_allergens = ?, dietary_restrictions = ?, updated_at = ?
		WHERE user_id = ?`,
		allergensJSON, restrictionsJSON, nowStr(), id,
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
		SELECT u.id, u.email, u.password_hash, u.created_at, u.updated_at,
		       COALESCE(p.excluded_allergens, '[]'),
		       COALESCE(p.dietary_restrictions, '[]')
		FROM users u
		LEFT JOIN user_preferences p ON p.user_id = u.id
		`+where, arg)

	var (
		id, email, passwordHash  string
		createdAt, updatedAt     string
		allergensJSON, dietsJSON string
	)
	if err := row.Scan(
		&id, &email, &passwordHash, &createdAt, &updatedAt,
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
	ct, _ := parseTime(createdAt)
	ut, _ := parseTime(updatedAt)

	return user.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		Preferences: user.Preferences{
			ExcludedAllergens:   allergens,
			DietaryRestrictions: diets,
		},
		CreatedAt: ct,
		UpdatedAt: ut,
	}, nil
}
