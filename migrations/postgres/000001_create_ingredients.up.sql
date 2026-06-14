CREATE TABLE IF NOT EXISTS ingredients (
    id                TEXT             NOT NULL PRIMARY KEY,
    name              TEXT             NOT NULL UNIQUE,
    display_name      TEXT             NOT NULL,
    food_group        TEXT             NOT NULL,
    allergens         TEXT             NOT NULL DEFAULT '[]',
    base_unit         TEXT             NOT NULL DEFAULT 'grams',
    unit_map          TEXT             NOT NULL DEFAULT '{}',
    calories_per_base DOUBLE PRECISION,
    protein_per_base  DOUBLE PRECISION,
    carbs_per_base    DOUBLE PRECISION,
    fat_per_base      DOUBLE PRECISION,
    created_at        TIMESTAMPTZ      NOT NULL,
    updated_at        TIMESTAMPTZ      NOT NULL
);
