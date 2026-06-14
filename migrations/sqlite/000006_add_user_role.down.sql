-- SQLite does not support DROP COLUMN before 3.35; recreate the table instead.
CREATE TABLE users_backup AS SELECT id, email, password_hash, created_at, updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_backup RENAME TO users;
