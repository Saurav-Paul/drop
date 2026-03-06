package upload

// This file handles the upload business logic — the most complex service in the app.
// Go equivalent of backend/api/upload/services/upload_service.py.

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/api/files"
	"github.com/Saurav-Paul/drop/internal/api/settings"
	"github.com/Saurav-Paul/drop/internal/config"
)

// expiryRegex matches expiry strings like "30m", "2h", "3d", "1w"
var expiryRegex = regexp.MustCompile(`^(\d+)([mhdw])$`)

// sizeRegex matches size strings like "100MB", "1GB", "500KB"
var sizeRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$`)

// sizeMultipliers maps size units to bytes
var sizeMultipliers = map[string]int64{
	"B":  1,
	"KB": 1024,
	"MB": 1024 * 1024,
	"GB": 1024 * 1024 * 1024,
	"TB": 1024 * 1024 * 1024 * 1024,
}

// GenerateCode creates a unique random code for the upload URL.
// Keeps generating until it finds one that doesn't exist in the DB.
//
// Python equivalent:
//
//	def generate_code(length=6):
//	    while True:
//	        code = secrets.token_urlsafe(length)[:length]
//	        if not files_repository.code_exists(code):
//	            return code
func GenerateCode(db *gorm.DB, length int) string {
	repo := files.NewRepository(db)

	for {
		// Generate random bytes and encode as URL-safe base64
		// This is like Python's secrets.token_urlsafe()
		buf := make([]byte, length)
		rand.Read(buf)
		code := base64.URLEncoding.EncodeToString(buf)[:length]

		if !repo.CodeExists(code) {
			return code
		}
	}
}

// ParseExpiry converts an expiry string like "30m", "2h", "3d" into a time.Time.
// Returns nil if the string is empty or invalid.
//
// Python equivalent:
//
//	def parse_expiry(expiry_str):
//	    match = re.match(r"^(\d+)([mhdw])$", expiry_str)
//	    return datetime.now(UTC) + deltas[unit]
func ParseExpiry(s string) *time.Time {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(strings.ToLower(s))
	matches := expiryRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}

	// matches[1] is the number, matches[2] is the unit
	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	// Map units to Go durations
	var duration time.Duration
	switch unit {
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	case "d":
		duration = time.Duration(value) * 24 * time.Hour
	case "w":
		duration = time.Duration(value) * 7 * 24 * time.Hour
	}

	t := time.Now().UTC().Add(duration)
	return &t
}

// ParseSize converts a size string like "100MB" into bytes.
// Returns 0 if the string is empty or invalid.
//
// Python equivalent:
//
//	def parse_size(size_str):
//	    match = re.match(r"^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$", size_str)
//	    return int(value * multipliers[unit])
func ParseSize(s string) int64 {
	if s == "" {
		return 0
	}

	s = strings.TrimSpace(strings.ToUpper(s))
	matches := sizeRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0
	}

	// Parse the numeric part as float (supports "1.5GB")
	value, _ := strconv.ParseFloat(matches[1], 64)
	unit := matches[2]

	multiplier, ok := sizeMultipliers[unit]
	if !ok {
		return 0
	}

	return int64(value * float64(multiplier))
}

// SaveUpload streams the request body to disk, validates limits, and creates a DB record.
// This is the core upload logic — the most complex function in the app.
//
// Python equivalent: upload_service.save_upload()
func SaveUpload(c echo.Context, db *gorm.DB, cfg *config.Config, filename string, expires string, maxDownloads *int, isAdmin bool) (*UploadResponse, error) {
	// Get current settings for limits and defaults
	settingsRepo := settings.NewRepository(db)
	settingsService := settings.NewService(settingsRepo)
	currentSettings := settingsService.GetAll()

	// --- Determine expiry ---
	var expiresAt *time.Time
	if expires != "" {
		expiresAt = ParseExpiry(expires)
	} else {
		expiresAt = ParseExpiry(currentSettings.DefaultExpiry)
	}

	// Enforce max expiry for non-admin uploads
	if !isAdmin && expiresAt != nil {
		maxExpiresAt := ParseExpiry(currentSettings.MaxExpiry)
		if maxExpiresAt != nil && expiresAt.After(*maxExpiresAt) {
			expiresAt = maxExpiresAt
		}
	}

	// Parse size limits from settings
	maxFileSize := ParseSize(currentSettings.MaxFileSize)
	storageLimit := ParseSize(currentSettings.StorageLimit)

	filesRepo := files.NewRepository(db)
	currentStorage := filesRepo.GetTotalStorage()

	// --- Check upload API key ---
	uploadKey := currentSettings.UploadAPIKey
	if uploadKey != "" && !isAdmin {
		providedKey := c.Request().Header.Get("X-Upload-Key")
		if providedKey != uploadKey {
			return nil, fmt.Errorf("permission: Invalid upload API key")
		}
	}

	// --- Stream request body to a temp file ---
	// Create a temp file in the files directory
	// os.CreateTemp is like Python's tempfile.NamedTemporaryFile(delete=False)
	tmpFile, err := os.CreateTemp(cfg.FilesDir, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	// defer runs when the function returns — like Python's try/finally
	defer func() {
		// If temp file still exists (wasn't moved to final location), clean it up
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	// Stream the body in chunks, tracking total size
	// This is like Python's: async for chunk in request.stream()
	var size int64
	buf := make([]byte, 32*1024) // 32KB read buffer

	for {
		// Read a chunk from the request body
		n, err := c.Request().Body.Read(buf)

		if n > 0 {
			size += int64(n)

			// Check file size limit during streaming (abort early)
			if maxFileSize > 0 && size > maxFileSize && !isAdmin {
				tmpFile.Close()
				return nil, fmt.Errorf("value: File exceeds max size of %s", currentSettings.MaxFileSize)
			}

			// Check storage limit during streaming
			if storageLimit > 0 && (currentStorage+size) > storageLimit && !isAdmin {
				tmpFile.Close()
				return nil, fmt.Errorf("value: Storage limit of %s would be exceeded", currentSettings.StorageLimit)
			}

			// Write the chunk to the temp file
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return nil, fmt.Errorf("failed to write: %w", writeErr)
			}
		}

		// io.EOF means we've read the entire body — this is normal, not an error
		if err == io.EOF {
			break
		}
		if err != nil {
			tmpFile.Close()
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
	}
	tmpFile.Close()

	// --- Generate code and move to final location ---
	code := GenerateCode(db, 6)

	// Create the code directory: <files_dir>/<code>/
	codeDir := filepath.Join(cfg.FilesDir, code)
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Move temp file to final path: <files_dir>/<code>/<filename>
	// os.Rename is like Python's shutil.move()
	finalPath := filepath.Join(codeDir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// --- Create DB record ---
	filesRepo.Create(code, filename, finalPath, size, maxDownloads, expiresAt)

	// --- Build download URL ---
	// Respect X-Forwarded-Proto header from reverse proxies (nginx, etc.)
	baseURL := fmt.Sprintf("%s://%s", c.Scheme(), c.Request().Host)
	proto := c.Request().Header.Get("X-Forwarded-Proto")
	if proto != "" {
		baseURL = fmt.Sprintf("%s://%s", proto, c.Request().Host)
	}
	url := fmt.Sprintf("%s/%s/%s", baseURL, code, filename)

	return &UploadResponse{
		URL:          url,
		Code:         code,
		Filename:     filename,
		Size:         size,
		ExpiresAt:    expiresAt,
		MaxDownloads: maxDownloads,
	}, nil
}
