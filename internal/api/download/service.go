package download

// This file handles the download business logic — validating files before serving.
// Go equivalent of backend/api/download/services/download_service.py.

import (
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/api/files"
)

// GetFileForDownload validates that a file can be downloaded and returns its path.
// Returns nil if the file doesn't exist, is expired, or has reached max downloads.
//
// Python equivalent:
//
//	def get_file_for_download(code, filename) -> Path | None:
//	    file_record = files_repository.get_by_code(code)
//	    # check filename, expiry, max_downloads, file exists on disk
//	    return filepath
func GetFileForDownload(db *gorm.DB, code, filename string) *string {
	repo := files.NewRepository(db)

	// Look up the file record in the database
	file := repo.GetByCode(code)
	if file == nil {
		return nil
	}

	// Check filename matches — prevents accessing a file with the wrong name
	if file.Filename != filename {
		return nil
	}

	// Check if the file has expired
	if file.ExpiresAt != nil {
		if time.Now().UTC().After(*file.ExpiresAt) {
			return nil
		}
	}

	// Check if max downloads has been reached
	if file.MaxDownloads != nil {
		if file.DownloadCount >= *file.MaxDownloads {
			return nil
		}
	}

	// Check the file actually exists on disk
	// (it could have been manually deleted or cleaned up)
	filepath := repo.GetFilepathByCode(code)
	if filepath == nil {
		return nil
	}

	// os.Stat checks if the file exists — like Python's Path.exists()
	if _, err := os.Stat(*filepath); os.IsNotExist(err) {
		return nil
	}

	return filepath
}

// RecordDownload increments the download counter for a file.
func RecordDownload(db *gorm.DB, code string) {
	repo := files.NewRepository(db)
	repo.IncrementDownload(code)
}
