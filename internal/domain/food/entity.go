package food

import "time"

type FoodIngredient struct {
	IngredientID string
	Amount       float64
	Unit         string
}

type NutritionInfo struct {
	CaloriesTotal      float64
	CaloriesPerPortion float64
	ProteinTotal       float64
	ProteinPerPortion  float64
	CarbsTotal         float64
	CarbsPerPortion    float64
	FatTotal           float64
	FatPerPortion      float64
}

type Food struct {
	ID          string
	Name        string // unique slug, e.g. "chinese-meal"
	DisplayName string
	Description string
	Portions    int // default 4
	Ingredients []FoodIngredient
	Recipe      []string
	Labels      []string
	Nutrition   NutritionInfo
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
