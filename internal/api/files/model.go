// Package files handles file metadata stored in the database.
// This file defines the GORM model — the Go equivalent of file_model.py.
package files

import "time"

// File is the GORM model for the "files" table.
// Each row represents an uploaded file with its metadata.
//
// Python equivalent:
//
//	class FileModel(Base):
//	    __tablename__ = "files"
//	    id = Column(Integer, primary_key=True, autoincrement=True)
//	    code = Column(String, unique=True, nullable=False, index=True)
//	    ...
type File struct {
	ID            uint       `gorm:"primaryKey;autoIncrement"`
	Code          string     `gorm:"uniqueIndex;not null"`          // Short code in the download URL (e.g. "aB3xYz")
	Filename      string     `gorm:"not null"`                      // Original filename from upload
	Filepath      string     `gorm:"not null"`                      // Full path to file on disk
	Size          int64      `gorm:"default:0"`                     // File size in bytes
	MaxDownloads  *int       `gorm:"column:max_downloads"`          // Optional download limit (nil = unlimited)
	DownloadCount int        `gorm:"column:download_count;default:0"` // Times downloaded so far
	ExpiresAt     *time.Time `gorm:"column:expires_at"`             // Optional expiry (nil = never)
	CreatedAt     time.Time  `gorm:"column:created_at"`             // Auto-set by DB default
}

// Note on pointer types (*int, *time.Time):
// These map to nullable columns in SQLite. A nil pointer means NULL in the DB.
// Non-pointer types (int, time.Time) can't be NULL — they always have a value.
//
// Python equivalent:
//   max_downloads = Column(Integer, nullable=True)    → *int  (pointer, can be nil)
//   download_count = Column(Integer, default=0)       → int   (always has a value)

func (File) TableName() string {
	return "files"
}
