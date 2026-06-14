package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

// ── fixture types (mirrors the JSON fixture schema) ───────────────────────────

type ingredientFixture struct {
	Name        string             `json:"name"`
	DisplayName string             `json:"display_name"`
	FoodGroup   string             `json:"food_group"`
	Allergens   []string           `json:"allergens"`
	BaseUnit    string             `json:"base_unit"`
	UnitMap     map[string]float64 `json:"unit_map"`
}

type foodIngredientFixture struct {
	IngredientName string  `json:"ingredient_name"`
	Amount         float64 `json:"amount"`
	Unit           string  `json:"unit"`
}

type foodFixture struct {
	Name        string                  `json:"name"`
	DisplayName string                  `json:"display_name"`
	Description string                  `json:"description"`
	Portions    int                     `json:"portions"`
	Recipe      []string                `json:"recipe"`
	Labels      []string                `json:"labels"`
	Ingredients []foodIngredientFixture `json:"ingredients"`
}

// ── ports (subset of the full repository interfaces) ─────────────────────────

type IngredientRepo interface {
	Create(ctx context.Context, i ingredient.Ingredient) (ingredient.Ingredient, error)
	GetByName(ctx context.Context, name string) (ingredient.Ingredient, error)
}

type FoodRepo interface {
	Create(ctx context.Context, f food.Food) (food.Food, error)
}

// ── Seeder ────────────────────────────────────────────────────────────────────

type Result struct {
	IngredientsInserted int
	IngredientsSkipped  int
	FoodsInserted       int
	FoodsSkipped        int
}

type Seeder struct {
	ingRepo  IngredientRepo
	foodRepo FoodRepo
	fixtFS   fs.FS // sub-FS rooted at the fixtures directory
}

func NewSeeder(ingRepo IngredientRepo, foodRepo FoodRepo, fixtureFS fs.FS) *Seeder {
	return &Seeder{ingRepo: ingRepo, foodRepo: foodRepo, fixtFS: fixtureFS}
}

func (s *Seeder) Seed(ctx context.Context) (Result, error) {
	var result Result

	ingByName, err := s.seedIngredients(ctx, &result)
	if err != nil {
		return Result{}, fmt.Errorf("seed ingredients: %w", err)
	}
	if err := s.seedFoods(ctx, ingByName, &result); err != nil {
		return Result{}, fmt.Errorf("seed foods: %w", err)
	}
	return result, nil
}

func (s *Seeder) seedIngredients(ctx context.Context, result *Result) (map[string]string, error) {
	data, err := fs.ReadFile(s.fixtFS, "ingredients.json")
	if err != nil {
		return nil, fmt.Errorf("read ingredients.json: %w", err)
	}

	var fixtures []ingredientFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		return nil, fmt.Errorf("parse ingredients.json: %w", err)
	}

	ingByName := make(map[string]string, len(fixtures))

	for _, fix := range fixtures {
		ing := ingredient.Ingredient{
			Name:        fix.Name,
			DisplayName: fix.DisplayName,
			FoodGroup:   ingredient.FoodGroup(fix.FoodGroup),
			Allergens:   stringsToAllergens(fix.Allergens),
			BaseUnit:    fix.BaseUnit,
			UnitMap:     ingredient.UnitMap(fix.UnitMap),
		}

		created, err := s.ingRepo.Create(ctx, ing)
		if errors.Is(err, domain.ErrAlreadyExists) {
			existing, lookupErr := s.ingRepo.GetByName(ctx, fix.Name)
			if lookupErr != nil {
				return nil, fmt.Errorf("lookup existing ingredient %q: %w", fix.Name, lookupErr)
			}
			ingByName[fix.Name] = existing.ID
			result.IngredientsSkipped++
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create ingredient %q: %w", fix.Name, err)
		}
		ingByName[created.Name] = created.ID
		result.IngredientsInserted++
	}
	return ingByName, nil
}

func (s *Seeder) seedFoods(ctx context.Context, ingByName map[string]string, result *Result) error {
	data, err := fs.ReadFile(s.fixtFS, "foods.json")
	if err != nil {
		return fmt.Errorf("read foods.json: %w", err)
	}

	var fixtures []foodFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		return fmt.Errorf("parse foods.json: %w", err)
	}

	for _, fix := range fixtures {
		fis, err := resolveFoodIngredients(fix.Ingredients, ingByName)
		if err != nil {
			return fmt.Errorf("food %q: %w", fix.Name, err)
		}

		portions := fix.Portions
		if portions == 0 {
			portions = 4
		}

		f := food.Food{
			Name:        fix.Name,
			DisplayName: fix.DisplayName,
			Description: fix.Description,
			Portions:    portions,
			Recipe:      fix.Recipe,
			Labels:      fix.Labels,
			Ingredients: fis,
		}

		if _, err := s.foodRepo.Create(ctx, f); errors.Is(err, domain.ErrAlreadyExists) {
			result.FoodsSkipped++
			continue
		} else if err != nil {
			return fmt.Errorf("create food %q: %w", fix.Name, err)
		}
		result.FoodsInserted++
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveFoodIngredients(fixes []foodIngredientFixture, ingByName map[string]string) ([]food.FoodIngredient, error) {
	fis := make([]food.FoodIngredient, 0, len(fixes))
	for _, fix := range fixes {
		id, ok := ingByName[fix.IngredientName]
		if !ok {
			return nil, fmt.Errorf("unknown ingredient %q — seed ingredients first", fix.IngredientName)
		}
		fis = append(fis, food.FoodIngredient{
			IngredientID: id,
			Amount:       fix.Amount,
			Unit:         fix.Unit,
		})
	}
	return fis, nil
}

func stringsToAllergens(ss []string) []ingredient.Allergen {
	out := make([]ingredient.Allergen, len(ss))
	for i, s := range ss {
		out[i] = ingredient.Allergen(s)
	}
	return out
}
