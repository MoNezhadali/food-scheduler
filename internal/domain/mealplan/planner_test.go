package mealplan_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/mealplan"
)

func makePool(n int) []food.Food {
	pool := make([]food.Food, n)
	for i := range pool {
		pool[i] = food.Food{ID: string(rune('A' + i)), Name: string(rune('a' + i))}
	}
	return pool
}

func TestPlan_ReturnsRequestedCount(t *testing.T) {
	pool := makePool(10)
	req := mealplan.Request{Count: 5}

	plan, err := mealplan.Plan(pool, req)

	require.NoError(t, err)
	assert.Len(t, plan.Foods, 5)
}

func TestPlan_NoRepeats(t *testing.T) {
	pool := makePool(10)
	req := mealplan.Request{Count: 10}

	plan, err := mealplan.Plan(pool, req)

	require.NoError(t, err)
	seen := make(map[string]bool)
	for _, f := range plan.Foods {
		assert.False(t, seen[f.ID], "food %q selected more than once", f.ID)
		seen[f.ID] = true
	}
}

func TestPlan_ExactFit(t *testing.T) {
	pool := makePool(4)
	req := mealplan.Request{Count: 4}

	plan, err := mealplan.Plan(pool, req)

	require.NoError(t, err)
	assert.Len(t, plan.Foods, 4)
}

func TestPlan_InsufficientPool(t *testing.T) {
	pool := makePool(3)
	req := mealplan.Request{Count: 5}

	_, err := mealplan.Plan(pool, req)

	assert.True(t, errors.Is(err, mealplan.ErrInsufficientFoods))
}

func TestPlan_ZeroCount(t *testing.T) {
	pool := makePool(5)
	req := mealplan.Request{Count: 0}

	plan, err := mealplan.Plan(pool, req)

	require.NoError(t, err)
	assert.Empty(t, plan.Foods)
}

func TestPlan_EmptyPool(t *testing.T) {
	req := mealplan.Request{Count: 1}

	_, err := mealplan.Plan([]food.Food{}, req)

	assert.True(t, errors.Is(err, mealplan.ErrInsufficientFoods))
}
