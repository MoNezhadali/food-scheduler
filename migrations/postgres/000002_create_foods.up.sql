CREATE TABLE IF NOT EXISTS foods (
    id                   TEXT             NOT NULL PRIMARY KEY,
    name                 TEXT             NOT NULL UNIQUE,
    display_name         TEXT             NOT NULL,
    description          TEXT             NOT NULL DEFAULT '',
    portions             INTEGER          NOT NULL DEFAULT 4,
    recipe               TEXT             NOT NULL DEFAULT '[]',
    labels               TEXT             NOT NULL DEFAULT '[]',
    calories_total       DOUBLE PRECISION NOT NULL DEFAULT 0,
    calories_per_portion DOUBLE PRECISION NOT NULL DEFAULT 0,
    protein_total        DOUBLE PRECISION NOT NULL DEFAULT 0,
    protein_per_portion  DOUBLE PRECISION NOT NULL DEFAULT 0,
    carbs_total          DOUBLE PRECISION NOT NULL DEFAULT 0,
    carbs_per_portion    DOUBLE PRECISION NOT NULL DEFAULT 0,
    fat_total            DOUBLE PRECISION NOT NULL DEFAULT 0,
    fat_per_portion      DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ      NOT NULL,
    updated_at           TIMESTAMPTZ      NOT NULL
);
