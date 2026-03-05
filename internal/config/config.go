// Package config loads application configuration from environment variables.
// This is the Go equivalent of backend/config.py in the Python version.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds all application configuration.
// In Go, we use a struct instead of module-level variables.
// Fields are exported (capitalized) so other packages can access them.
type Config struct {
	DataDir      string // Root directory for all persistent data (DB + files)
	FilesDir     string // Subdirectory where uploaded files are stored
	AdminUser    string // Admin username for authentication
	AdminPass    string // Admin password for authentication
	AdminEnabled bool   // True only when both AdminUser and AdminPass are set
}

// Load reads configuration from environment variables and creates required directories.
// This is called once at startup from main.go.
//
// Go equivalent of Python's:
//
//	DATA_DIR = Path(os.environ.get("DATA_DIR", ...))
//	DROP_ADMIN_USER = os.environ.get("DROP_ADMIN_USER", "").strip()
func Load() (*Config, error) {
	// os.Getenv returns "" if the variable is not set — same as Python's os.environ.get("KEY", "")
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data" // Default to ./data in the current working directory
	}

	filesDir := filepath.Join(dataDir, "files")

	// strings.TrimSpace is like Python's .strip()
	adminUser := strings.TrimSpace(os.Getenv("DROP_ADMIN_USER"))
	adminPass := strings.TrimSpace(os.Getenv("DROP_ADMIN_PASS"))

	// Create directories if they don't exist
	// os.MkdirAll is like Python's Path.mkdir(parents=True, exist_ok=True)
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, err
	}

	return &Config{
		DataDir:      dataDir,
		FilesDir:     filesDir,
		AdminUser:    adminUser,
		AdminPass:    adminPass,
		AdminEnabled: adminUser != "" && adminPass != "",
	}, nil
}
