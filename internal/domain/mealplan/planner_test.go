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

func labeledPool(specs []struct {
	id    string
	label string
}) []food.Food {
	pool := make([]food.Food, len(specs))
	for i, s := range specs {
		pool[i] = food.Food{ID: s.id, Name: s.id, Labels: []string{s.label}}
	}
	return pool
}

// ── existing tests (no preferences) ──────────────────────────────────────────

func TestPlan_ReturnsRequestedCount(t *testing.T) {
	plan, err := mealplan.Plan(makePool(10), mealplan.Request{Count: 5})
	require.NoError(t, err)
	assert.Len(t, plan.Foods, 5)
}

func TestPlan_NoRepeats(t *testing.T) {
	plan, err := mealplan.Plan(makePool(10), mealplan.Request{Count: 10})
	require.NoError(t, err)
	seen := make(map[string]bool)
	for _, f := range plan.Foods {
		assert.False(t, seen[f.ID], "food %q selected more than once", f.ID)
		seen[f.ID] = true
	}
}

func TestPlan_ExactFit(t *testing.T) {
	plan, err := mealplan.Plan(makePool(4), mealplan.Request{Count: 4})
	require.NoError(t, err)
	assert.Len(t, plan.Foods, 4)
}

func TestPlan_InsufficientPool(t *testing.T) {
	_, err := mealplan.Plan(makePool(3), mealplan.Request{Count: 5})
	assert.True(t, errors.Is(err, mealplan.ErrInsufficientFoods))
}

func TestPlan_ZeroCount(t *testing.T) {
	plan, err := mealplan.Plan(makePool(5), mealplan.Request{Count: 0})
	require.NoError(t, err)
	assert.Empty(t, plan.Foods)
}

func TestPlan_EmptyPool(t *testing.T) {
	_, err := mealplan.Plan([]food.Food{}, mealplan.Request{Count: 1})
	assert.True(t, errors.Is(err, mealplan.ErrInsufficientFoods))
}

// ── preference tests ──────────────────────────────────────────────────────────

func TestPlan_MinBeef(t *testing.T) {
	pool := labeledPool([]struct{ id, label string }{
		{"beef1", "beef"}, {"beef2", "beef"},
		{"other1", "chicken"}, {"other2", "chicken"}, {"other3", "chicken"},
	})
	req := mealplan.Request{Count: 3, Preferences: mealplan.Preferences{MinBeef: 1}}
	for range 20 { // run many times: result must always contain ≥1 beef
		plan, err := mealplan.Plan(pool, req)
		require.NoError(t, err)
		assert.Len(t, plan.Foods, 3)
		beefCount := 0
		for _, f := range plan.Foods {
			if f.Labels[0] == "beef" {
				beefCount++
			}
		}
		assert.GreaterOrEqual(t, beefCount, 1, "expected ≥1 beef food")
	}
}

func TestPlan_MinBeefAndChicken(t *testing.T) {
	pool := labeledPool([]struct{ id, label string }{
		{"b1", "beef"}, {"b2", "beef"},
		{"c1", "chicken"}, {"c2", "chicken"},
		{"f1", "fish"},
	})
	req := mealplan.Request{
		Count: 4,
		Preferences: mealplan.Preferences{MinBeef: 1, MinChicken: 1},
	}
	for range 20 {
		plan, err := mealplan.Plan(pool, req)
		require.NoError(t, err)
		assert.Len(t, plan.Foods, 4)
		var beefCount, chickenCount int
		for _, f := range plan.Foods {
			switch f.Labels[0] {
			case "beef":
				beefCount++
			case "chicken":
				chickenCount++
			}
		}
		assert.GreaterOrEqual(t, beefCount, 1)
		assert.GreaterOrEqual(t, chickenCount, 1)
	}
}

func TestPlan_PreferenceNoRepeats(t *testing.T) {
	pool := labeledPool([]struct{ id, label string }{
		{"b1", "beef"}, {"c1", "chicken"}, {"f1", "fish"}, {"o1", "other"}, {"o2", "other"},
	})
	req := mealplan.Request{
		Count: 4,
		Preferences: mealplan.Preferences{MinBeef: 1, MinChicken: 1, MinFish: 1},
	}
	for range 20 {
		plan, err := mealplan.Plan(pool, req)
		require.NoError(t, err)
		seen := make(map[string]bool)
		for _, f := range plan.Foods {
			assert.False(t, seen[f.ID], "food %q repeated", f.ID)
			seen[f.ID] = true
		}
	}
}

func TestPlan_InsufficientForPreference(t *testing.T) {
	pool := labeledPool([]struct{ id, label string }{
		{"b1", "beef"}, {"o1", "other"}, {"o2", "other"},
	})
	req := mealplan.Request{Count: 3, Preferences: mealplan.Preferences{MinBeef: 2}}
	_, err := mealplan.Plan(pool, req)
	assert.True(t, errors.Is(err, mealplan.ErrInsufficientFoods))
}
