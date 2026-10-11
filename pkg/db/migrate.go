package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// MigrateUp executes pending database migrations.
func MigrateUp(connStr, migrationsDir string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("migrate: failed to open database: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: failed to set dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("migrate: failed to run migrations up: %w", err)
	}

	return nil
}

// MigrateDown rolls back the last database migration.
func MigrateDown(connStr, migrationsDir string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("migrate: failed to open database: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: failed to set dialect: %w", err)
	}

	if err := goose.Down(db, migrationsDir); err != nil {
		return fmt.Errorf("migrate: failed to run migration down: %w", err)
	}

	return nil
}
