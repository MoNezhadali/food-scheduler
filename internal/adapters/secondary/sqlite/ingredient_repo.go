package sqliteadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

type IngredientRepo struct {
	db *sql.DB
}

func NewIngredientRepo(db *sql.DB) *IngredientRepo {
	return &IngredientRepo{db: db}
}

const ingredientColumns = `
	id, name, display_name, food_group, allergens, base_unit, unit_map,
	calories_per_base, protein_per_base, carbs_per_base, fat_per_base,
	created_at, updated_at`

func (r *IngredientRepo) List(ctx context.Context, filter ingredient.Filter) ([]ingredient.Ingredient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+ingredientColumns+` FROM ingredients ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list ingredients: %w", err)
	}
	defer rows.Close()

	var all []ingredient.Ingredient
	for rows.Next() {
		ing, err := scanIngredient(rows.Scan)
		if err != nil {
			return nil, err
		}
		all = append(all, ing)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applyIngredientFilter(all, filter), nil
}

func (r *IngredientRepo) GetByID(ctx context.Context, id string) (ingredient.Ingredient, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+ingredientColumns+` FROM ingredients WHERE id = ?`, id)
	ing, err := scanIngredient(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return ingredient.Ingredient{}, domain.ErrNotFound
	}
	return ing, err
}

func (r *IngredientRepo) GetByName(ctx context.Context, name string) (ingredient.Ingredient, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+ingredientColumns+` FROM ingredients WHERE name = ?`, name)
	ing, err := scanIngredient(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return ingredient.Ingredient{}, domain.ErrNotFound
	}
	return ing, err
}

func (r *IngredientRepo) GetByIDs(ctx context.Context, ids []string) ([]ingredient.Ingredient, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT` + ingredientColumns + ` FROM ingredients WHERE id IN (` + inPlaceholders(len(ids)) + `)`
	rows, err := r.db.QueryContext(ctx, q, stringsToAny(ids)...)
	if err != nil {
		return nil, fmt.Errorf("get ingredients by ids: %w", err)
	}
	defer rows.Close()

	var result []ingredient.Ingredient
	for rows.Next() {
		ing, err := scanIngredient(rows.Scan)
		if err != nil {
			return nil, err
		}
		result = append(result, ing)
	}
	return result, rows.Err()
}

func (r *IngredientRepo) Create(ctx context.Context, i ingredient.Ingredient) (ingredient.Ingredient, error) {
	i.ID = uuid.NewString()
	ts := nowStr()
	i.CreatedAt, _ = parseTime(ts)
	i.UpdatedAt = i.CreatedAt

	allergensJSON, err := toJSON(allergensToStrings(i.Allergens))
	if err != nil {
		return ingredient.Ingredient{}, err
	}
	unitMapJSON, err := toJSON(map[string]float64(i.UnitMap))
	if err != nil {
		return ingredient.Ingredient{}, err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ingredients
			(id, name, display_name, food_group, allergens, base_unit, unit_map,
			 calories_per_base, protein_per_base, carbs_per_base, fat_per_base,
			 created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		i.ID, i.Name, i.DisplayName, string(i.FoodGroup), allergensJSON,
		i.BaseUnit, unitMapJSON,
		i.Nutrition.CaloriesPerBase, i.Nutrition.ProteinPerBase,
		i.Nutrition.CarbsPerBase, i.Nutrition.FatPerBase,
		ts, ts,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ingredient.Ingredient{}, fmt.Errorf("%w: ingredient name %q", domain.ErrAlreadyExists, i.Name)
		}
		return ingredient.Ingredient{}, fmt.Errorf("create ingredient: %w", err)
	}
	return i, nil
}

func (r *IngredientRepo) Update(ctx context.Context, i ingredient.Ingredient) (ingredient.Ingredient, error) {
	ts := nowStr()
	i.UpdatedAt, _ = parseTime(ts)

	allergensJSON, err := toJSON(allergensToStrings(i.Allergens))
	if err != nil {
		return ingredient.Ingredient{}, err
	}
	unitMapJSON, err := toJSON(map[string]float64(i.UnitMap))
	if err != nil {
		return ingredient.Ingredient{}, err
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE ingredients SET
			name = ?, display_name = ?, food_group = ?, allergens = ?,
			base_unit = ?, unit_map = ?,
			calories_per_base = ?, protein_per_base = ?,
			carbs_per_base = ?, fat_per_base = ?,
			updated_at = ?
		WHERE id = ?`,
		i.Name, i.DisplayName, string(i.FoodGroup), allergensJSON,
		i.BaseUnit, unitMapJSON,
		i.Nutrition.CaloriesPerBase, i.Nutrition.ProteinPerBase,
		i.Nutrition.CarbsPerBase, i.Nutrition.FatPerBase,
		ts, i.ID,
	)
	if err != nil {
		return ingredient.Ingredient{}, fmt.Errorf("update ingredient: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ingredient.Ingredient{}, domain.ErrNotFound
	}
	return i, nil
}

func (r *IngredientRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ingredients WHERE id = ?`, id)
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return fmt.Errorf("%w: ingredient is used by one or more foods", domain.ErrInvalidInput)
		}
		return fmt.Errorf("delete ingredient: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *IngredientRepo) ListMissingNutrition(ctx context.Context) ([]ingredient.Ingredient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+ingredientColumns+` FROM ingredients WHERE calories_per_base IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list missing nutrition: %w", err)
	}
	defer rows.Close()

	var result []ingredient.Ingredient
	for rows.Next() {
		ing, err := scanIngredient(rows.Scan)
		if err != nil {
			return nil, err
		}
		result = append(result, ing)
	}
	return result, rows.Err()
}

func (r *IngredientRepo) UpdateNutrition(ctx context.Context, id string, n ingredient.NutritionInfo) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ingredients SET
			calories_per_base = ?, protein_per_base = ?,
			carbs_per_base = ?, fat_per_base = ?,
			updated_at = ?
		WHERE id = ?`,
		n.CaloriesPerBase, n.ProteinPerBase, n.CarbsPerBase, n.FatPerBase,
		nowStr(), id,
	)
	if err != nil {
		return fmt.Errorf("update nutrition: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanIngredient(scan func(...any) error) (ingredient.Ingredient, error) {
	var (
		id, name, displayName, foodGroup string
		allergensJSON, baseUnit, unitMapJSON string
		cal, prot, carb, fat             *float64
		createdAt, updatedAt             string
	)
	if err := scan(
		&id, &name, &displayName, &foodGroup,
		&allergensJSON, &baseUnit, &unitMapJSON,
		&cal, &prot, &carb, &fat,
		&createdAt, &updatedAt,
	); err != nil {
		return ingredient.Ingredient{}, err
	}

	var allergenStrs []string
	if err := fromJSON(allergensJSON, &allergenStrs); err != nil {
		return ingredient.Ingredient{}, fmt.Errorf("parse allergens: %w", err)
	}
	var unitMap map[string]float64
	if err := fromJSON(unitMapJSON, &unitMap); err != nil {
		return ingredient.Ingredient{}, fmt.Errorf("parse unit_map: %w", err)
	}
	ct, _ := parseTime(createdAt)
	ut, _ := parseTime(updatedAt)

	return ingredient.Ingredient{
		ID:          id,
		Name:        name,
		DisplayName: displayName,
		FoodGroup:   ingredient.FoodGroup(foodGroup),
		Allergens:   stringsToAllergens(allergenStrs),
		BaseUnit:    baseUnit,
		UnitMap:     ingredient.UnitMap(unitMap),
		Nutrition: ingredient.NutritionInfo{
			CaloriesPerBase: cal,
			ProteinPerBase:  prot,
			CarbsPerBase:    carb,
			FatPerBase:      fat,
		},
		CreatedAt: ct,
		UpdatedAt: ut,
	}, nil
}

func applyIngredientFilter(all []ingredient.Ingredient, f ingredient.Filter) []ingredient.Ingredient {
	var result []ingredient.Ingredient
	for _, ing := range all {
		if f.FoodGroup != nil && ing.FoodGroup != *f.FoodGroup {
			continue
		}
		if len(f.AllergenFree) > 0 {
			hasExcluded := false
			allergenSet := make(map[ingredient.Allergen]bool, len(ing.Allergens))
			for _, a := range ing.Allergens {
				allergenSet[a] = true
			}
			for _, excl := range f.AllergenFree {
				if allergenSet[excl] {
					hasExcluded = true
					break
				}
			}
			if hasExcluded {
				continue
			}
		}
		if f.Search != nil && *f.Search != "" {
			s := strings.ToLower(*f.Search)
			if !strings.Contains(strings.ToLower(ing.Name), s) &&
				!strings.Contains(strings.ToLower(ing.DisplayName), s) {
				continue
			}
		}
		result = append(result, ing)
	}
	return result
}

func allergensToStrings(as []ingredient.Allergen) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = string(a)
	}
	return out
}

func stringsToAllergens(ss []string) []ingredient.Allergen {
	out := make([]ingredient.Allergen, len(ss))
	for i, s := range ss {
		out[i] = ingredient.Allergen(s)
	}
	return out
}
