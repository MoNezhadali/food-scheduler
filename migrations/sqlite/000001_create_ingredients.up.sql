CREATE TABLE IF NOT EXISTS ingredients (
    id                TEXT NOT NULL PRIMARY KEY,
    name              TEXT NOT NULL UNIQUE,
    display_name      TEXT NOT NULL,
    food_group        TEXT NOT NULL,
    allergens         TEXT NOT NULL DEFAULT '[]',
    base_unit         TEXT NOT NULL DEFAULT 'grams',
    unit_map          TEXT NOT NULL DEFAULT '{}',
    calories_per_base REAL,
    protein_per_base  REAL,
    carbs_per_base    REAL,
    fat_per_base      REAL,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
