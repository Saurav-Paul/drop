// Package database sets up the GORM connection and runs migrations.
// This is the Go equivalent of backend/database.py in the Python version.
package database

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/config"
)

// Embed the migrations directory into the binary at compile time.
// This means the SQL files are baked into the binary — no need to ship them separately.
// The //go:embed directive tells the Go compiler to include these files.
//
//go:embed migrations
var migrations embed.FS

// Setup opens the SQLite database via GORM and runs any pending Goose migrations.
// Returns the *gorm.DB instance used throughout the app.
//
// Go equivalent of Python's:
//
//	engine = create_engine(DATABASE_URL, ...)
//	SessionLocal = sessionmaker(bind=engine)
//	run_migrations()  # Alembic upgrade head
func Setup(cfg *config.Config) (*gorm.DB, error) {
	// Build the database file path: <data_dir>/drop.db
	dbPath := filepath.Join(cfg.DataDir, "drop.db")

	// Open the SQLite database with GORM
	// gorm.Config{} uses sensible defaults — similar to SQLAlchemy's create_engine()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get the underlying *sql.DB from GORM — needed for Goose migrations
	// GORM wraps the standard database/sql package; we can access it when needed
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Run migrations using the embedded SQL files
	if err := runMigrations(sqlDB); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// runMigrations applies pending Goose migrations from the embedded filesystem.
// This runs automatically on every startup — like Alembic's "upgrade head".
func runMigrations(db *sql.DB) error {
	// Tell Goose to read migration files from the embedded filesystem
	// instead of looking for them on disk
	goose.SetBaseFS(migrations)

	// Set the database dialect — we're using SQLite
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	// Run all pending migrations
	// "migrations" is the directory path within the embedded filesystem
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	return nil
}
