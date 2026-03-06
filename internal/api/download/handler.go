package download

// This file defines the HTTP route handler for file downloads.
// Go equivalent of backend/api/download/controllers/download_controller.py.

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Register wires up the download route.
// This is a catch-all GET route — must be registered AFTER specific API routes.
//
// Usage in main.go:
//
//	download.Register(e, db)
func Register(e *echo.Echo, db *gorm.DB) {
	handler := &Handler{db: db}

	// GET /:code/:filename — matches download URLs like /aB3xYz/file.txt
	e.GET("/:code/:filename", handler.downloadFile)
}

// Handler holds dependencies for download handlers.
type Handler struct {
	db *gorm.DB
}

// downloadFile streams a file download with correct headers.
//
// Python equivalent:
//
//	@router.get("/{code}/{filename:path}")
//	async def download_file(code: str, filename: str):
//	    filepath = download_service.get_file_for_download(code, filename)
//	    download_service.record_download(code)
//	    return StreamingResponse(iterfile(), headers=...)
func (h *Handler) downloadFile(c echo.Context) error {
	code := c.Param("code")
	filename := c.Param("filename")

	// Validate the file — checks expiry, max downloads, file exists on disk
	fp := GetFileForDownload(h.db, code, filename)
	if fp == nil {
		return echo.NewHTTPError(http.StatusNotFound, "File not found or expired")
	}

	// Record the download (increment counter)
	RecordDownload(h.db, code)

	// Determine content type from the file extension
	// mime.TypeByExtension is like Python's mimetypes.guess_type()
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream" // Default for unknown types
	}

	// Set Content-Disposition header to trigger a download in the browser
	// "attachment" means the browser will download instead of displaying inline
	c.Response().Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename))

	// c.File() serves the file with proper Content-Length and streaming.
	// Under the hood, Go uses http.ServeFile which is more efficient than
	// the Python version's manual 1MB chunk streaming — it handles:
	//   - Content-Length header automatically
	//   - Range requests (resume downloads)
	//   - Efficient OS-level file copying (sendfile syscall on Linux)
	return c.File(*fp)
}
