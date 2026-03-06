// Package cleanup handles expired file removal and orphaned directory cleanup.
// Go equivalent of backend/cleanup.py.
//
// In the Python version, cleanup runs as a system cron job inside the Docker container.
// In Go, we use robfig/cron to run it in-process — no system cron needed.
// This makes the Docker image simpler (just the binary, no cron daemon).
package cleanup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/api/files"
	"github.com/Saurav-Paul/drop/internal/api/settings"
	"github.com/Saurav-Paul/drop/internal/config"
)

// RunCleanup deletes expired files, max-downloaded files, and orphaned directories.
// Returns the number of files cleaned up.
//
// Python equivalent:
//
//	def run_cleanup():
//	    # delete expired, delete max_downloads_reached, remove orphaned dirs
//	    settings_repository.set("last_cleanup", now)
//	    return deleted_count
func RunCleanup(db *gorm.DB, cfg *config.Config) int {
	filesRepo := files.NewRepository(db)
	settingsRepo := settings.NewRepository(db)
	deleted := 0

	// 1. Delete expired files
	expired := filesRepo.GetExpired()
	for _, f := range expired {
		deleteFile(filesRepo, f.Code, cfg)
		deleted++
	}

	// 2. Delete files that reached max downloads
	maxedOut := filesRepo.GetMaxDownloadsReached()
	for _, f := range maxedOut {
		deleteFile(filesRepo, f.Code, cfg)
		deleted++
	}

	// 3. Remove orphaned directories on disk
	// These are directories in the files dir that don't have a matching DB record
	deleted += removeOrphanedDirs(filesRepo, cfg)

	// 4. Record the cleanup timestamp in settings
	now := time.Now().UTC().Format(time.RFC3339)
	settingsRepo.Set("last_cleanup", now)

	if deleted > 0 {
		log.Printf("Cleanup: removed %d file(s)", deleted)
	}

	return deleted
}

// deleteFile removes a file from both the database and disk.
//
// Python equivalent:
//
//	def _delete_file(code):
//	    files_repository.delete_by_code(code)
//	    shutil.rmtree(file_path.parent, ignore_errors=True)
func deleteFile(repo *files.Repository, code string, cfg *config.Config) {
	// Get filepath before deleting the DB record
	fp := repo.GetFilepathByCode(code)

	// Delete from DB
	repo.DeleteByCode(code)

	// Delete from disk
	if fp != nil {
		dir := filepath.Dir(*fp)
		os.RemoveAll(dir)
	}
}

// removeOrphanedDirs scans the files directory for directories that exist on disk
// but don't have a corresponding record in the database.
//
// Python equivalent:
//
//	# Scan FILES_DIR for orphaned directories
//	for entry in FILES_DIR.iterdir():
//	    if entry.is_dir() and not files_repository.code_exists(entry.name):
//	        shutil.rmtree(entry)
func removeOrphanedDirs(repo *files.Repository, cfg *config.Config) int {
	deleted := 0

	// os.ReadDir lists all entries in a directory — like Python's Path.iterdir()
	entries, err := os.ReadDir(cfg.FilesDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		// Skip files, only check directories (each upload creates a directory named after its code)
		if !entry.IsDir() {
			continue
		}

		// If the directory name doesn't match any code in the DB, it's orphaned
		if !repo.CodeExists(entry.Name()) {
			os.RemoveAll(filepath.Join(cfg.FilesDir, entry.Name()))
			deleted++
		}
	}

	return deleted
}

// StartCron starts an in-process cron scheduler that runs cleanup every 12 hours.
// Returns the *cron.Cron instance so the caller can stop it on shutdown.
//
// Python Dockerfile equivalent:
//
//	RUN echo "0 */12 * * * ... python backend/cleanup.py" > /etc/cron.d/drop-cleanup
//
// In Go, we don't need system cron — robfig/cron runs inside the process.
func StartCron(db *gorm.DB, cfg *config.Config) *cron.Cron {
	// cron.New() creates a new scheduler
	c := cron.New()

	// AddFunc schedules a function to run on a cron expression
	// "0 */12 * * *" means "at minute 0 of every 12th hour"
	// Same schedule as the Python version's system cron
	c.AddFunc("0 */12 * * *", func() {
		RunCleanup(db, cfg)
	})

	// Start the scheduler in a background goroutine
	// Goroutines are like lightweight threads — they run concurrently without blocking
	c.Start()

	fmt.Println("Cleanup cron started (every 12 hours)")

	return c
}
