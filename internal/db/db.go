package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQLite string

//go:embed schema_mysql.sql
var schemaMySQL string

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	var db *sql.DB
	var err error
	var dbType string

	// Determine database type based on DSN format
	// MySQL DSN examples: user:password@tcp(host:port)/dbname, user:password@/dbname
	// SQLite DSN: file path (e.g., data/kotatsu.db, /path/to/db.sqlite, :memory:)

	// Simple heuristic: if DSN contains '@' it's likely MySQL
	isMySQL := strings.Contains(dsn, "@")

	if isMySQL {
		// MySQL database
		dbType = "mysql"
		db, err = sql.Open("mysql", dsn)
	} else {
		// SQLite database - ensure directory exists (unless it's :memory:)
		dbType = "sqlite"
		if dsn != ":memory:" {
			dir := filepath.Dir(dsn)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create database directory: %w", err)
			}
		}

		// Add SQLite pragmas via DSN to ensure they apply to all connections
		if !strings.Contains(dsn, "?") {
			dsn += "?"
		} else {
			dsn += "&"
		}

		// Configure WAL, Foreign Keys, Busy Timeout via DSN
		// modernc.org/sqlite uses _pragma query parameters
		pragmas := []string{
			"_pragma=foreign_keys(1)",
			"_pragma=journal_mode(WAL)",
			"_pragma=busy_timeout(30000)",
			"_pragma=synchronous(NORMAL)",
			"_pragma=cache_size(-20000)",
			"_pragma=temp_store(MEMORY)",
		}
		dsn += strings.Join(pragmas, "&")

		db, err = sql.Open("sqlite", dsn)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Apply SQLite-specific optimizations
	if dbType == "sqlite" {
		// SQLite needs more connections to handle nested queries (N+1) in fetchFavourites
		// and concurrent requests, preventing deadlock (e.g. Reader holds Conn1, needs Conn2).
		db.SetMaxOpenConns(25)
	}

	if err := initSchema(db, dbType); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DB{db}, nil
}

func initSchema(db *sql.DB, dbType string) error {
	var schema string
	if dbType == "mysql" {
		schema = schemaMySQL
	} else {
		schema = schemaSQLite
	}

	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
func (db *DB) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, nickname, favourites_sync_timestamp, history_sync_timestamp, password_reset_token_hash, password_reset_token_expires_at FROM users WHERE email = ?`
	row := db.QueryRow(query, email)
	err := row.Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Nickname,
		&user.FavouritesSyncTimestamp, &user.HistorySyncTimestamp,
		&user.PasswordResetTokenHash, &user.PasswordResetTokenExpires,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) SetPasswordResetToken(userID int64, tokenHash string, expiresAt int64) error {
	query := `UPDATE users SET password_reset_token_hash = ?, password_reset_token_expires_at = ? WHERE id = ?`
	_, err := db.Exec(query, tokenHash, expiresAt, userID)
	return err
}

func (db *DB) GetUserByResetToken(tokenHash string) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, nickname, favourites_sync_timestamp, history_sync_timestamp, password_reset_token_hash, password_reset_token_expires_at FROM users WHERE password_reset_token_hash = ?`
	row := db.QueryRow(query, tokenHash)
	err := row.Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Nickname,
		&user.FavouritesSyncTimestamp, &user.HistorySyncTimestamp,
		&user.PasswordResetTokenHash, &user.PasswordResetTokenExpires,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) UpdatePassword(userID int64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ? WHERE id = ?`
	_, err := db.Exec(query, passwordHash, userID)
	return err
}

func (db *DB) ClearResetToken(userID int64) error {
	query := `UPDATE users SET password_reset_token_hash = NULL, password_reset_token_expires_at = NULL WHERE id = ?`
	_, err := db.Exec(query, userID)
	return err
}

func (db *DB) UserExists(id int64) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (db *DB) GetUserByID(id int64) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, nickname, favourites_sync_timestamp, history_sync_timestamp, password_reset_token_hash, password_reset_token_expires_at FROM users WHERE id = ?`
	row := db.QueryRow(query, id)
	err := row.Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Nickname,
		&user.FavouritesSyncTimestamp, &user.HistorySyncTimestamp,
		&user.PasswordResetTokenHash, &user.PasswordResetTokenExpires,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
