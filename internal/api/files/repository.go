package files

// This file is the data access layer — all database queries for files.
// Go equivalent of backend/api/files/repositories/files_repository.py.

import (
	"time"

	"gorm.io/gorm"
)

// Repository provides database operations for the files table.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository with the given database connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ListAll returns all files ordered by creation date (newest first).
//
// Python equivalent:
//
//	def list_all():
//	    models = session.query(FileModel).order_by(FileModel.created_at.desc()).all()
//	    return [_model_to_dto(m) for m in models]
func (r *Repository) ListAll() []FileResponse {
	var models []File
	r.db.Order("created_at DESC").Find(&models)

	// Convert each model to a DTO — like a list comprehension in Python
	results := make([]FileResponse, len(models))
	for i, m := range models {
		results[i] = ToResponse(m)
	}
	return results
}

// GetByCode finds a file by its code, or returns nil if not found.
//
// Python equivalent:
//
//	def get_by_code(code):
//	    model = session.query(FileModel).filter_by(code=code).first()
//	    return _model_to_dto(model) if model else None
func (r *Repository) GetByCode(code string) *FileResponse {
	var model File
	result := r.db.Where("code = ?", code).First(&model)
	if result.Error != nil {
		return nil
	}
	resp := ToResponse(model)
	return &resp
}

// GetFilepathByCode returns the filepath for a given code, or nil if not found.
func (r *Repository) GetFilepathByCode(code string) *string {
	var model File
	result := r.db.Where("code = ?", code).First(&model)
	if result.Error != nil {
		return nil
	}
	return &model.Filepath
}

// Create inserts a new file record and returns the response.
//
// Python equivalent:
//
//	def create(code, filename, filepath, size, max_downloads, expires_at):
//	    model = FileModel(...)
//	    session.add(model)
//	    session.commit()
func (r *Repository) Create(code, filename, filepath string, size int64, maxDownloads *int, expiresAt *time.Time) FileResponse {
	model := File{
		Code:         code,
		Filename:     filename,
		Filepath:     filepath,
		Size:         size,
		MaxDownloads: maxDownloads,
		ExpiresAt:    expiresAt,
	}
	r.db.Create(&model)
	return ToResponse(model)
}

// CodeExists checks if a code already exists in the database.
func (r *Repository) CodeExists(code string) bool {
	var count int64
	r.db.Model(&File{}).Where("code = ?", code).Count(&count)
	return count > 0
}

// IncrementDownload bumps the download counter for a file.
//
// Python equivalent:
//
//	def increment_download(code):
//	    model.download_count = (model.download_count or 0) + 1
//	    session.commit()
func (r *Repository) IncrementDownload(code string) {
	// gorm.Expr("...") lets us write raw SQL expressions
	// This is safer than read-modify-write because it's atomic
	r.db.Model(&File{}).Where("code = ?", code).
		Update("download_count", gorm.Expr("download_count + 1"))
}

// DeleteByCode removes a file record by its code.
func (r *Repository) DeleteByCode(code string) bool {
	result := r.db.Where("code = ?", code).Delete(&File{})
	// RowsAffected tells us if a row was actually deleted
	return result.RowsAffected > 0
}

// GetExpired returns all files past their expiry time.
func (r *Repository) GetExpired() []FileResponse {
	var models []File
	now := time.Now().UTC()
	r.db.Where("expires_at IS NOT NULL AND expires_at <= ?", now).Find(&models)

	results := make([]FileResponse, len(models))
	for i, m := range models {
		results[i] = ToResponse(m)
	}
	return results
}

// GetMaxDownloadsReached returns files that have hit their download limit.
func (r *Repository) GetMaxDownloadsReached() []FileResponse {
	var models []File
	r.db.Where("max_downloads IS NOT NULL AND download_count >= max_downloads").Find(&models)

	results := make([]FileResponse, len(models))
	for i, m := range models {
		results[i] = ToResponse(m)
	}
	return results
}

// GetTotalStorage returns the sum of all file sizes in bytes.
func (r *Repository) GetTotalStorage() int64 {
	var total *int64
	r.db.Model(&File{}).Select("COALESCE(SUM(size), 0)").Scan(&total)
	if total == nil {
		return 0
	}
	return *total
}
