package pgadapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
)

type FoodRepo struct {
	db *sql.DB
}

func NewFoodRepo(db *sql.DB) *FoodRepo {
	return &FoodRepo{db: db}
}

const foodColumns = `
	id, name, display_name, description, portions, recipe, labels,
	calories_total, calories_per_portion,
	protein_total, protein_per_portion,
	carbs_total, carbs_per_portion,
	fat_total, fat_per_portion,
	created_at, updated_at`

func (r *FoodRepo) List(ctx context.Context, filter food.Filter) ([]food.Food, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT`+foodColumns+` FROM foods ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list foods: %w", err)
	}
	defer rows.Close()

	var foods []food.Food
	var ids []string
	for rows.Next() {
		f, err := scanFood(rows.Scan)
		if err != nil {
			return nil, err
		}
		foods = append(foods, f)
		ids = append(ids, f.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(foods) == 0 {
		return nil, nil
	}

	fiMap, err := r.loadFoodIngredients(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range foods {
		foods[i].Ingredients = fiMap[foods[i].ID]
	}

	if len(filter.ExcludeAllergens) > 0 {
		ingAllergens, err := r.ingredientAllergens(ctx, allIngredientIDs(foods))
		if err != nil {
			return nil, err
		}
		foods = applyFoodFilter(foods, filter, ingAllergens)
	} else {
		foods = applyFoodFilter(foods, filter, nil)
	}
	return foods, nil
}

func (r *FoodRepo) GetByID(ctx context.Context, id string) (food.Food, error) {
	row := r.db.QueryRowContext(ctx, `SELECT`+foodColumns+` FROM foods WHERE id = $1`, id)
	f, err := scanFood(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return food.Food{}, domain.ErrNotFound
	}
	if err != nil {
		return food.Food{}, err
	}

	fiMap, err := r.loadFoodIngredients(ctx, []string{id})
	if err != nil {
		return food.Food{}, err
	}
	f.Ingredients = fiMap[id]
	return f, nil
}

func (r *FoodRepo) GetByIDs(ctx context.Context, ids []string) ([]food.Food, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT` + foodColumns + ` FROM foods WHERE id IN (` + inPlaceholders(len(ids)) + `)`
	rows, err := r.db.QueryContext(ctx, q, stringsToAny(ids)...)
	if err != nil {
		return nil, fmt.Errorf("get foods by ids: %w", err)
	}
	defer rows.Close()

	var foods []food.Food
	for rows.Next() {
		f, err := scanFood(rows.Scan)
		if err != nil {
			return nil, err
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	fiMap, err := r.loadFoodIngredients(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range foods {
		foods[i].Ingredients = fiMap[foods[i].ID]
	}
	return foods, nil
}

func (r *FoodRepo) Create(ctx context.Context, f food.Food) (food.Food, error) {
	f.ID = uuid.NewString()
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now

	recipeJSON, err := toJSON(f.Recipe)
	if err != nil {
		return food.Food{}, err
	}
	labelsJSON, err := toJSON(f.Labels)
	if err != nil {
		return food.Food{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return food.Food{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO foods
			(id, name, display_name, description, portions, recipe, labels,
			 calories_total, calories_per_portion,
			 protein_total, protein_per_portion,
			 carbs_total, carbs_per_portion,
			 fat_total, fat_per_portion,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		f.ID, f.Name, f.DisplayName, f.Description, f.Portions, recipeJSON, labelsJSON,
		f.Nutrition.CaloriesTotal, f.Nutrition.CaloriesPerPortion,
		f.Nutrition.ProteinTotal, f.Nutrition.ProteinPerPortion,
		f.Nutrition.CarbsTotal, f.Nutrition.CarbsPerPortion,
		f.Nutrition.FatTotal, f.Nutrition.FatPerPortion,
		now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return food.Food{}, fmt.Errorf("%w: food name %q", domain.ErrAlreadyExists, f.Name)
		}
		return food.Food{}, fmt.Errorf("insert food: %w", err)
	}

	if err := insertFoodIngredients(ctx, tx, f.ID, f.Ingredients); err != nil {
		return food.Food{}, err
	}
	if err := tx.Commit(); err != nil {
		return food.Food{}, err
	}
	return f, nil
}

func (r *FoodRepo) Update(ctx context.Context, f food.Food) (food.Food, error) {
	now := time.Now().UTC()
	f.UpdatedAt = now

	recipeJSON, err := toJSON(f.Recipe)
	if err != nil {
		return food.Food{}, err
	}
	labelsJSON, err := toJSON(f.Labels)
	if err != nil {
		return food.Food{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return food.Food{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE foods SET
			name = $1, display_name = $2, description = $3, portions = $4,
			recipe = $5, labels = $6,
			calories_total = $7, calories_per_portion = $8,
			protein_total = $9, protein_per_portion = $10,
			carbs_total = $11, carbs_per_portion = $12,
			fat_total = $13, fat_per_portion = $14,
			updated_at = $15
		WHERE id = $16`,
		f.Name, f.DisplayName, f.Description, f.Portions, recipeJSON, labelsJSON,
		f.Nutrition.CaloriesTotal, f.Nutrition.CaloriesPerPortion,
		f.Nutrition.ProteinTotal, f.Nutrition.ProteinPerPortion,
		f.Nutrition.CarbsTotal, f.Nutrition.CarbsPerPortion,
		f.Nutrition.FatTotal, f.Nutrition.FatPerPortion,
		now, f.ID,
	)
	if err != nil {
		return food.Food{}, fmt.Errorf("update food: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return food.Food{}, domain.ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM food_ingredients WHERE food_id = $1`, f.ID); err != nil {
		return food.Food{}, err
	}
	if err := insertFoodIngredients(ctx, tx, f.ID, f.Ingredients); err != nil {
		return food.Food{}, err
	}
	if err := tx.Commit(); err != nil {
		return food.Food{}, err
	}
	return f, nil
}

func (r *FoodRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM foods WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete food: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanFood(scan func(...any) error) (food.Food, error) {
	var (
		id, name, displayName, description string
		portions                           int
		recipeJSON, labelsJSON             string
		calTot, calPor                     float64
		protTot, protPor                   float64
		carbTot, carbPor                   float64
		fatTot, fatPor                     float64
		createdAt, updatedAt               time.Time
	)
	if err := scan(
		&id, &name, &displayName, &description, &portions, &recipeJSON, &labelsJSON,
		&calTot, &calPor, &protTot, &protPor, &carbTot, &carbPor, &fatTot, &fatPor,
		&createdAt, &updatedAt,
	); err != nil {
		return food.Food{}, err
	}

	var recipe, labels []string
	if err := json.Unmarshal([]byte(recipeJSON), &recipe); err != nil {
		return food.Food{}, fmt.Errorf("parse recipe: %w", err)
	}
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return food.Food{}, fmt.Errorf("parse labels: %w", err)
	}

	return food.Food{
		ID: id, Name: name, DisplayName: displayName, Description: description,
		Portions: portions, Recipe: recipe, Labels: labels,
		Nutrition: food.NutritionInfo{
			CaloriesTotal: calTot, CaloriesPerPortion: calPor,
			ProteinTotal: protTot, ProteinPerPortion: protPor,
			CarbsTotal: carbTot, CarbsPerPortion: carbPor,
			FatTotal: fatTot, FatPerPortion: fatPor,
		},
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func (r *FoodRepo) loadFoodIngredients(ctx context.Context, foodIDs []string) (map[string][]food.FoodIngredient, error) {
	if len(foodIDs) == 0 {
		return map[string][]food.FoodIngredient{}, nil
	}
	q := `SELECT food_id, ingredient_id, amount, unit FROM food_ingredients WHERE food_id IN (` +
		inPlaceholders(len(foodIDs)) + `)`
	rows, err := r.db.QueryContext(ctx, q, stringsToAny(foodIDs)...)
	if err != nil {
		return nil, fmt.Errorf("load food_ingredients: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]food.FoodIngredient)
	for rows.Next() {
		var foodID string
		var fi food.FoodIngredient
		if err := rows.Scan(&foodID, &fi.IngredientID, &fi.Amount, &fi.Unit); err != nil {
			return nil, err
		}
		result[foodID] = append(result[foodID], fi)
	}
	return result, rows.Err()
}

func (r *FoodRepo) ingredientAllergens(ctx context.Context, ids []string) (map[string][]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT id, allergens FROM ingredients WHERE id IN (` + inPlaceholders(len(ids)) + `)`
	rows, err := r.db.QueryContext(ctx, q, stringsToAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var id, allergensJSON string
		if err := rows.Scan(&id, &allergensJSON); err != nil {
			return nil, err
		}
		var allergens []string
		if err := json.Unmarshal([]byte(allergensJSON), &allergens); err != nil {
			return nil, err
		}
		result[id] = allergens
	}
	return result, rows.Err()
}

func insertFoodIngredients(ctx context.Context, tx *sql.Tx, foodID string, fis []food.FoodIngredient) error {
	for _, fi := range fis {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO food_ingredients (food_id, ingredient_id, amount, unit) VALUES ($1,$2,$3,$4)`,
			foodID, fi.IngredientID, fi.Amount, fi.Unit,
		); err != nil {
			return fmt.Errorf("insert food_ingredient %s/%s: %w", foodID, fi.IngredientID, err)
		}
	}
	return nil
}

func allIngredientIDs(foods []food.Food) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, f := range foods {
		for _, fi := range f.Ingredients {
			if !seen[fi.IngredientID] {
				seen[fi.IngredientID] = true
				ids = append(ids, fi.IngredientID)
			}
		}
	}
	return ids
}

func applyFoodFilter(foods []food.Food, filter food.Filter, ingAllergens map[string][]string) []food.Food {
	var result []food.Food
	for _, f := range foods {
		if len(filter.Labels) > 0 {
			labelSet := make(map[string]bool, len(f.Labels))
			for _, l := range f.Labels {
				labelSet[l] = true
			}
			match := true
			for _, req := range filter.Labels {
				if !labelSet[req] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		if len(filter.ExcludeAllergens) > 0 && ingAllergens != nil {
			exclSet := make(map[string]bool, len(filter.ExcludeAllergens))
			for _, a := range filter.ExcludeAllergens {
				exclSet[a] = true
			}
			excluded := false
			for _, fi := range f.Ingredients {
				for _, a := range ingAllergens[fi.IngredientID] {
					if exclSet[a] {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			if excluded {
				continue
			}
		}
		if filter.Search != nil && *filter.Search != "" {
			s := strings.ToLower(*filter.Search)
			if !strings.Contains(strings.ToLower(f.Name), s) &&
				!strings.Contains(strings.ToLower(f.DisplayName), s) &&
				!strings.Contains(strings.ToLower(f.Description), s) {
				continue
			}
		}
		result = append(result, f)
	}
	return result
}
