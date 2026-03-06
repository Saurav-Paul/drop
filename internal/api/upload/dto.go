package upload

// This file defines the response struct for uploads.
// Go equivalent of backend/api/upload/dto/upload.py.

import "time"

// UploadResponse is the JSON returned after a successful upload.
// Field names must match the Python version exactly for CLI compatibility.
//
// Python equivalent:
//
//	class UploadResponse(BaseModel):
//	    url: str
//	    code: str
//	    filename: str
//	    size: int
//	    expires_at: datetime | None = None
//	    max_downloads: int | None = None
type UploadResponse struct {
	URL          string     `json:"url"`
	Code         string     `json:"code"`
	Filename     string     `json:"filename"`
	Size         int64      `json:"size"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxDownloads *int       `json:"max_downloads"`
}
