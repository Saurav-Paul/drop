-- +goose Up
-- This migration creates the initial database schema.
-- Matches the Alembic migration from the Python version exactly,
-- so existing databases work without changes.

-- Files table: stores metadata for each uploaded file
CREATE TABLE IF NOT EXISTS files (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    code           TEXT    NOT NULL UNIQUE,     -- Short random code used in download URLs
    filename       TEXT    NOT NULL,            -- Original filename from the upload
    filepath       TEXT    NOT NULL,            -- Full path to the file on disk
    size           INTEGER DEFAULT 0,           -- File size in bytes
    max_downloads  INTEGER,                     -- Optional download limit (NULL = unlimited)
    download_count INTEGER DEFAULT 0,           -- How many times the file has been downloaded
    expires_at     DATETIME,                    -- Optional expiry timestamp (NULL = never)
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index on code for fast lookups (downloads, admin API)
CREATE UNIQUE INDEX IF NOT EXISTS ix_files_code ON files(code);

-- Settings table: key-value store for server configuration
-- Used for default_expiry, max_file_size, storage_limit, etc.
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS settings;
