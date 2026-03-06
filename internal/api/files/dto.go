package files

// This file defines the request/response structs for the files API.
// Go equivalent of backend/api/files/dto/file.py.

import "time"

// FileResponse is the JSON shape returned by the files API.
// Field names match the Python version exactly for API compatibility.
//
// Python equivalent:
//
//	class FileResponse(BaseModel):
//	    id: int
//	    code: str
//	    filename: str
//	    size: int
//	    max_downloads: int | None = None
//	    download_count: int
//	    expires_at: datetime | None = None
//	    created_at: datetime
type FileResponse struct {
	ID            uint       `json:"id"`
	Code          string     `json:"code"`
	Filename      string     `json:"filename"`
	Size          int64      `json:"size"`
	MaxDownloads  *int       `json:"max_downloads"`  // nil → null in JSON
	DownloadCount int        `json:"download_count"`
	ExpiresAt     *time.Time `json:"expires_at"`     // nil → null in JSON
	CreatedAt     time.Time  `json:"created_at"`
}

// ToResponse converts a File (GORM model) to a FileResponse (API DTO).
// This keeps the database model separate from the API response shape.
//
// Python equivalent:
//
//	def _model_to_dto(model: FileModel) -> FileResponse:
//	    return FileResponse(id=model.id, code=model.code, ...)
func ToResponse(f File) FileResponse {
	return FileResponse{
		ID:            f.ID,
		Code:          f.Code,
		Filename:      f.Filename,
		Size:          f.Size,
		MaxDownloads:  f.MaxDownloads,
		DownloadCount: f.DownloadCount,
		ExpiresAt:     f.ExpiresAt,
		CreatedAt:     f.CreatedAt,
	}
}
