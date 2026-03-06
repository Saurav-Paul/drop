package files

// This file contains the business logic for file management.
// Go equivalent of backend/api/files/services/files_service.py.

import (
	"os"
	"path/filepath"
)

// Service handles business logic for files.
type Service struct {
	repo *Repository
}

// NewService creates a new Service with the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListFiles returns all files.
func (s *Service) ListFiles() []FileResponse {
	return s.repo.ListAll()
}

// GetFile returns a single file by code, or nil if not found.
func (s *Service) GetFile(code string) *FileResponse {
	return s.repo.GetByCode(code)
}

// DeleteFile removes a file from DB and disk.
//
// Python equivalent:
//
//	def delete_file(code):
//	    filepath = files_repository.get_filepath_by_code(code)
//	    files_repository.delete_by_code(code)
//	    shutil.rmtree(file_path.parent, ignore_errors=True)
func (s *Service) DeleteFile(code string) bool {
	// Get the filepath before deleting the DB record
	fp := s.repo.GetFilepathByCode(code)
	if fp == nil {
		return false
	}

	// Delete from DB
	s.repo.DeleteByCode(code)

	// Delete from disk — remove the code directory (contains the file)
	// filepath.Dir() gets the parent directory, like Python's Path.parent
	// os.RemoveAll() is like shutil.rmtree() — removes directory and all contents
	dir := filepath.Dir(*fp)
	os.RemoveAll(dir)

	return true
}

// StatsResponse holds aggregate statistics for the dashboard.
type StatsResponse struct {
	TotalFiles     int   `json:"total_files"`
	TotalStorage   int64 `json:"total_storage"`
	TotalDownloads int   `json:"total_downloads"`
}

// GetStats returns aggregate file statistics.
//
// Python equivalent:
//
//	def get_stats():
//	    return {
//	        "total_files": len(files),
//	        "total_storage": total_storage,
//	        "total_downloads": sum(f.download_count for f in files),
//	    }
func (s *Service) GetStats() StatsResponse {
	files := s.repo.ListAll()
	totalStorage := s.repo.GetTotalStorage()

	totalDownloads := 0
	for _, f := range files {
		totalDownloads += f.DownloadCount
	}

	return StatsResponse{
		TotalFiles:     len(files),
		TotalStorage:   totalStorage,
		TotalDownloads: totalDownloads,
	}
}
