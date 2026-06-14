CREATE TABLE IF NOT EXISTS user_preferences (
    user_id              TEXT NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    excluded_allergens   TEXT NOT NULL DEFAULT '[]',
    dietary_restrictions TEXT NOT NULL DEFAULT '[]',
    updated_at           TEXT NOT NULL
);
