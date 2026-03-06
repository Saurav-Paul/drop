package upload

// This file defines the HTTP route handler for file uploads.
// Go equivalent of backend/api/upload/controllers/upload_controller.py.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/config"
	mw "github.com/Saurav-Paul/drop/internal/middleware"
)

// Register wires up the upload domain and attaches the route.
// Upload is a catch-all PUT route — it must be registered AFTER specific API routes
// so it doesn't intercept requests meant for /api/settings, /api/files, etc.
//
// Usage in main.go:
//
//	upload.Register(e, db, cfg)
func Register(e *echo.Echo, db *gorm.DB, cfg *config.Config) {
	handler := &Handler{db: db, cfg: cfg}

	// PUT /:filename — catch-all for file uploads
	// Note: we use "*" to match filenames that might contain slashes (e.g. "subdir/file.txt")
	// The Python version uses {filename:path} which also matches multi-segment paths
	e.PUT("/*", handler.uploadFile)
}

// Handler holds dependencies for upload handlers.
type Handler struct {
	db  *gorm.DB
	cfg *config.Config
}

// uploadFile handles file uploads via streaming PUT request.
//
// Python equivalent:
//
//	@router.put("/{filename:path}", response_model=UploadResponse)
//	async def upload_file(request: Request, filename: str):
//	    admin = is_admin(request)
//	    expires = request.headers.get("X-Expires")
//	    max_downloads_str = request.headers.get("X-Max-Downloads")
//	    return await upload_service.save_upload(...)
func (h *Handler) uploadFile(c echo.Context) error {
	// Extract the filename from the URL path
	// c.Param("*") gets everything after the "/" in the catch-all route
	// strings.TrimPrefix removes the leading "/" that Echo includes
	filename := strings.TrimPrefix(c.Param("*"), "/")
	if filename == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Filename is required")
	}

	// Check if the requester is an admin (affects limit enforcement)
	// Upload itself is public, but admin uploads bypass size/expiry limits
	isAdmin := mw.IsAdmin(c, h.cfg)

	// Read optional headers
	expires := c.Request().Header.Get("X-Expires")

	// Parse max downloads header — convert string to *int
	var maxDownloads *int
	if maxDlStr := c.Request().Header.Get("X-Max-Downloads"); maxDlStr != "" {
		if val, err := strconv.Atoi(maxDlStr); err == nil {
			maxDownloads = &val
		}
	}

	// Call the service to handle the upload
	result, err := SaveUpload(c, h.db, h.cfg, filename, expires, maxDownloads, isAdmin)
	if err != nil {
		errMsg := err.Error()

		// Map error types to HTTP status codes
		// The service returns errors with prefixes like "value:" and "permission:"
		// to indicate the type, similar to Python raising ValueError or PermissionError
		if strings.HasPrefix(errMsg, "value:") {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, strings.TrimPrefix(errMsg, "value: "))
		}
		if strings.HasPrefix(errMsg, "permission:") {
			return echo.NewHTTPError(http.StatusForbidden, strings.TrimPrefix(errMsg, "permission: "))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Upload failed")
	}

	return c.JSON(http.StatusOK, result)
}
