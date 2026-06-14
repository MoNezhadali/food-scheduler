package ingredient

import "time"

type FoodGroup string

const (
	FoodGroupFruitsAndVegetables FoodGroup = "fruits-and-vegetables"
	FoodGroupMeatsAndProteins    FoodGroup = "meats-and-proteins"
	FoodGroupDairy               FoodGroup = "dairy"
	FoodGroupGrainsAndStarches   FoodGroup = "grains-and-starches"
	FoodGroupOilsAndFats         FoodGroup = "oils-and-fats"
	FoodGroupSpicesAndSeasonings FoodGroup = "spices-and-seasonings"
	FoodGroupNutsAndSeeds        FoodGroup = "nuts-and-seeds"
	FoodGroupCannedAndJarred     FoodGroup = "canned-and-jarred-goods"
	FoodGroupBeverages           FoodGroup = "beverages"
	FoodGroupOther               FoodGroup = "other"
)

type Allergen string

const (
	AllergenGluten    Allergen = "gluten"
	AllergenDairy     Allergen = "dairy"
	AllergenNuts      Allergen = "nuts"
	AllergenEggs      Allergen = "eggs"
	AllergenFish      Allergen = "fish"
	AllergenShellfish Allergen = "shellfish"
	AllergenSoy       Allergen = "soy"
	AllergenPoultry   Allergen = "poultry"
	AllergenRedMeat   Allergen = "red-meat"
)

// UnitMap maps unit names to their conversion factor relative to the base unit.
// Example for rice (base_unit = "grams"): {"grams": 1, "cup": 200, "cups": 200}
// means 1 cup = 200 grams. To convert: amount_in_base = amount * factor.
type UnitMap map[string]float64

type NutritionInfo struct {
	CaloriesPerBase *float64 // kcal per 1 base unit (nil = unknown)
	ProteinPerBase  *float64 // grams per 1 base unit
	CarbsPerBase    *float64
	FatPerBase      *float64
}

type Ingredient struct {
	ID          string
	Name        string // unique slug, e.g. "chicken-breast"
	DisplayName string
	FoodGroup   FoodGroup
	Allergens   []Allergen
	BaseUnit    string // e.g. "grams"
	UnitMap     UnitMap
	Nutrition   NutritionInfo
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ToBaseAmount converts an amount in the given unit to the ingredient's base unit.
// Falls back to the raw amount if the unit is not in the map.
func (i Ingredient) ToBaseAmount(amount float64, unit string) float64 {
	if factor, ok := i.UnitMap[unit]; ok && factor > 0 {
		return amount * factor
	}
	return amount
}
