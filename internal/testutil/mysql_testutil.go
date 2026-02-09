package testutil

import (
	"os"
	"testing"

	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
)

// SetupMySQLTestDB initializes a MySQL-backed DB for integration tests.
// It skips tests when MYSQL_TEST_DSN is not set.
func SetupMySQLTestDB(t *testing.T) *db.DB {
	t.Helper()

	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set; skipping MySQL integration tests")
	}

	database, err := db.New(dsn)
	if err != nil {
		t.Fatalf("failed to init mysql test db: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	resetMySQLTables(t, database)
	return database
}

func resetMySQLTables(t *testing.T, database *db.DB) {
	t.Helper()

	stmts := []string{
		"SET FOREIGN_KEY_CHECKS=0",
		"TRUNCATE TABLE manga_tags",
		"TRUNCATE TABLE tags",
		"TRUNCATE TABLE favourites",
		"TRUNCATE TABLE history",
		"TRUNCATE TABLE categories",
		"TRUNCATE TABLE manga",
		"TRUNCATE TABLE users",
		"SET FOREIGN_KEY_CHECKS=1",
	}

	for _, stmt := range stmts {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatalf("mysql reset failed on %q: %v", stmt, err)
		}
	}
}

