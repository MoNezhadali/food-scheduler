CREATE TABLE IF NOT EXISTS food_ingredients (
    food_id       TEXT             NOT NULL REFERENCES foods(id) ON DELETE CASCADE,
    ingredient_id TEXT             NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    amount        DOUBLE PRECISION NOT NULL,
    unit          TEXT             NOT NULL,
    PRIMARY KEY (food_id, ingredient_id)
);

CREATE INDEX IF NOT EXISTS idx_food_ingredients_ingredient ON food_ingredients(ingredient_id);
