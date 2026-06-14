package mealplan

import (
	"errors"
	"math/rand/v2"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
)

var ErrInsufficientFoods = errors.New("not enough foods match the given constraints")

// Plan selects req.Count foods at random from pool without repetition.
// The pool must already be filtered by allergen/dietary restrictions before
// being passed in. Preference minimums are satisfied first by selecting from
// foods labelled "beef", "chicken", "fish", or "pork"; remaining slots are
// filled from any unused pool food.
func Plan(pool []food.Food, req Request) (MealPlan, error) {
	if len(pool) < req.Count {
		return MealPlan{}, ErrInsufficientFoods
	}

	selected := make([]food.Food, 0, req.Count)
	used := make(map[string]bool, req.Count)

	// Satisfy minimum-count constraints in order.
	constraints := []struct {
		label string
		min   int
	}{
		{"beef", req.Preferences.MinBeef},
		{"chicken", req.Preferences.MinChicken},
		{"fish", req.Preferences.MinFish},
		{"pork", req.Preferences.MinPork},
	}
	for _, c := range constraints {
		if c.min <= 0 {
			continue
		}
		bucket := foodsWithLabel(pool, c.label, used)
		if len(bucket) < c.min {
			return MealPlan{}, ErrInsufficientFoods
		}
		for _, idx := range rand.Perm(len(bucket))[:c.min] {
			f := bucket[idx]
			selected = append(selected, f)
			used[f.ID] = true
		}
	}

	// Fill remaining slots from the rest of the pool.
	remaining := req.Count - len(selected)
	if remaining > 0 {
		leftover := make([]food.Food, 0, len(pool)-len(used))
		for _, f := range pool {
			if !used[f.ID] {
				leftover = append(leftover, f)
			}
		}
		if len(leftover) < remaining {
			return MealPlan{}, ErrInsufficientFoods
		}
		for _, idx := range rand.Perm(len(leftover))[:remaining] {
			selected = append(selected, leftover[idx])
		}
	}

	return MealPlan{Foods: selected}, nil
}

func foodsWithLabel(pool []food.Food, label string, exclude map[string]bool) []food.Food {
	var result []food.Food
	for _, f := range pool {
		if exclude[f.ID] {
			continue
		}
		for _, l := range f.Labels {
			if l == label {
				result = append(result, f)
				break
			}
		}
	}
	return result
}
