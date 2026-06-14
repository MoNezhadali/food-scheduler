package mealplan

import (
	"errors"
	"math/rand/v2"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
)

var ErrInsufficientFoods = errors.New("not enough foods match the given constraints")

// Plan selects req.Count foods at random from pool without repetition.
// The pool must already be filtered by restrictions before being passed in.
// Preference satisfaction (min_beef, min_chicken, etc.) is added in Phase 8.
func Plan(pool []food.Food, req Request) (MealPlan, error) {
	if len(pool) < req.Count {
		return MealPlan{}, ErrInsufficientFoods
	}
	indices := rand.Perm(len(pool))
	selected := make([]food.Food, req.Count)
	for i := range req.Count {
		selected[i] = pool[indices[i]]
	}
	return MealPlan{Foods: selected}, nil
}
