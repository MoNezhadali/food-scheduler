package mealplan

import "github.com/MoNezhadali/foodscheduler/internal/domain/food"

type Restrictions struct {
	ExcludeAllergens    []string
	DietaryRestrictions []string // e.g. "vegetarian", "no-pork"
}

// Preferences expresses minimum-count constraints by protein type.
// A zero value means no constraint for that type.
type Preferences struct {
	MinBeef    int
	MinChicken int
	MinFish    int
	MinPork    int
}

type Request struct {
	Count              int
	UseUserPreferences bool
	ExtraRestrictions  Restrictions
	Preferences        Preferences
}

type MealPlan struct {
	Foods []food.Food
}
