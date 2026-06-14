package shoppinglist

type Item struct {
	IngredientID string
	Name         string // slug
	DisplayName  string
	TotalAmount  float64
	Unit         string // base unit of the ingredient
	FoodGroup    string
}

type ShoppingList struct {
	Categories map[string][]Item // keyed by food_group
	TotalItems int
}
