package sqliteadapter_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	err = database.RunMigrations(db, migrations.SQLiteFS, "sqlite")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}
