package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dsn)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DB{db}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
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
