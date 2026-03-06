// Package pages handles all HTML page routes — login, logout, dashboard, settings,
// and HTMX endpoints for file delete, cleanup, and manual upload.
// Go equivalent of backend/api/pages/controllers/pages_controller.py.
package pages

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/api/files"
	"github.com/Saurav-Paul/drop/internal/api/settings"
	"github.com/Saurav-Paul/drop/internal/api/upload"
	"github.com/Saurav-Paul/drop/internal/cleanup"
	"github.com/Saurav-Paul/drop/internal/config"
	mw "github.com/Saurav-Paul/drop/internal/middleware"
)

// Handler holds dependencies for all page routes.
type Handler struct {
	db   *gorm.DB
	cfg  *config.Config
	tmpl *template.Template // Parsed template set — all templates loaded at startup
}

// templateFuncs returns the custom functions available inside templates.
// These replace Jinja2 filters like {{ value | timeago }} with Go's {{ timeago .Value }}.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// timeago converts a time.Time to a human-readable relative string like "5m ago".
		// Python equivalent: the _timeago() filter in pages_controller.py
		"timeago": func(t time.Time) string {
			now := time.Now().UTC()
			// Ensure both times are in UTC for comparison
			t = t.UTC()
			seconds := now.Sub(t).Seconds()

			if seconds < 60 {
				return "just now"
			}
			if seconds < 3600 {
				m := int(seconds / 60)
				return fmt.Sprintf("%dm ago", m)
			}
			if seconds < 86400 {
				h := int(seconds / 3600)
				return fmt.Sprintf("%dh ago", h)
			}
			if seconds < 604800 {
				d := int(seconds / 86400)
				return fmt.Sprintf("%dd ago", d)
			}
			return t.Format("2006-01-02")
		},

		// filesize converts bytes to a human-readable string like "1.5 MB".
		// Python equivalent: the _filesize() filter in pages_controller.py
		"filesize": func(size int64) string {
			if size < 1024 {
				return fmt.Sprintf("%d B", size)
			}
			if size < 1024*1024 {
				return fmt.Sprintf("%.1f KB", float64(size)/1024)
			}
			if size < 1024*1024*1024 {
				return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
			}
			return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
		},

		// deref dereferences a pointer — used in templates for optional fields.
		// Example: {{deref .MaxDownloads}} prints the int value if non-nil.
		// Go templates can't do *ptr, so we need this helper.
		"deref": func(v interface{}) interface{} {
			switch val := v.(type) {
			case *int:
				if val != nil {
					return *val
				}
			case *time.Time:
				if val != nil {
					return *val
				}
			}
			return nil
		},
	}
}

// Register wires up the pages domain and attaches all HTML routes.
// Unlike API domains, pages use the root Echo instance (not a group)
// because the routes are at various paths (/, /login, /settings, etc.).
//
// Usage in main.go:
//
//	pages.Register(e, db, cfg)
func Register(e *echo.Echo, db *gorm.DB, cfg *config.Config) {
	// Parse all templates at startup.
	// template.Must() panics if parsing fails — we want to crash early if templates are broken.
	// Funcs() must be called BEFORE ParseGlob() so the functions are available during parsing.
	tmpl := template.Must(
		template.New("").Funcs(templateFuncs()).ParseGlob("templates/*.html"),
	)

	handler := &Handler{db: db, cfg: cfg, tmpl: tmpl}

	// --- Auth routes ---
	e.GET("/login", handler.loginPage)
	e.POST("/login", handler.loginSubmit)
	e.GET("/logout", handler.logout)

	// --- Dashboard ---
	e.GET("/", handler.dashboard)

	// --- Settings pages ---
	e.GET("/settings", handler.settingsPage)
	e.POST("/settings", handler.settingsSubmit)

	// --- HTMX endpoints ---
	// These return HTML fragments, not full pages.
	// The HTMX library on the frontend swaps these fragments into the DOM.
	e.DELETE("/api/files/:code/htmx", handler.htmxDeleteFile)
	e.POST("/cleanup", handler.htmxCleanup)
	e.POST("/upload", handler.manualUpload)

	// --- Static files ---
	// Serves files from the "static/" directory at "/static/" URL path.
	// Like Python's StaticFiles(directory="static")
	e.Static("/static", "static")
}

// renderPage is a helper that renders a page template inside the base layout.
// Go's html/template doesn't have Jinja2's {% extends %}, so we manually
// execute the base template which calls {{template "content" .}} to include the page.
//
// Steps:
//  1. Clone the base template set (so we can add the page template without mutating the original)
//  2. Parse the page-specific template into the clone
//  3. Execute "base.html" which pulls in "content" from the page template
func (h *Handler) renderPage(c echo.Context, name string, data map[string]interface{}, statusCodes ...int) error {
	// Clone creates a copy of the parsed template set
	// We need to clone because each page defines {{define "content"}} and
	// we don't want them overwriting each other in the shared template set.
	t, err := h.tmpl.Clone()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Template clone error: %v", err))
	}

	// Parse the page template — it defines {{define "content"}}...{{end}}
	_, err = t.ParseFiles("templates/" + name)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Template parse error: %v", err))
	}

	// Render to a buffer first so errors don't produce partial output.
	// If we wrote directly to the response and the template errored halfway,
	// the user would see broken HTML with a 200 status.
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Template exec error: %v", err))
	}

	// Set status code (default 200).
	// statusCodes is a variadic param — allows optional status: renderPage(c, name, data, 401)
	status := http.StatusOK
	if len(statusCodes) > 0 {
		status = statusCodes[0]
	}

	return c.HTMLBlob(status, buf.Bytes())
}

// --- Auth routes ---

// loginPage renders the login form.
// If admin is not enabled or user is already logged in, redirect to dashboard.
//
// Python equivalent: @router.get("/login") async def login_page(request):
func (h *Handler) loginPage(c echo.Context) error {
	// If admin is disabled or already authenticated, go straight to dashboard
	if !h.cfg.AdminEnabled || mw.IsAdmin(c, h.cfg) {
		return c.Redirect(http.StatusFound, "/")
	}

	return h.renderPage(c, "login.html", map[string]interface{}{
		"Title": "Login — Drop",
		"Error": "",
	})
}

// loginSubmit handles login form submission.
// Validates credentials and sets a session cookie on success.
//
// Python equivalent: @router.post("/login") async def login_submit(request, username, password):
func (h *Handler) loginSubmit(c echo.Context) error {
	if !h.cfg.AdminEnabled {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	// c.FormValue() reads form fields — like FastAPI's Form(...)
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Constant-time comparison to prevent timing attacks
	// subtle.ConstantTimeCompare returns 1 if equal, 0 if not
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(h.cfg.AdminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(h.cfg.AdminPass)) == 1

	if userOK && passOK {
		// Credentials valid — set the session cookie and redirect to dashboard.
		// http.StatusSeeOther (303) tells the browser to GET the redirect URL
		// (prevents form resubmission on refresh).
		cookie := new(http.Cookie)
		cookie.Name = mw.CookieName
		cookie.Value = mw.MakeToken(h.cfg.AdminUser, h.cfg.AdminPass)
		cookie.HttpOnly = true               // Not accessible via JavaScript (XSS protection)
		cookie.SameSite = http.SameSiteLaxMode // Prevents CSRF from third-party sites
		cookie.MaxAge = 60 * 60 * 24 * 30     // 30 days, same as Python version
		cookie.Path = "/"                      // Cookie applies to all paths
		c.SetCookie(cookie)

		return c.Redirect(http.StatusSeeOther, "/")
	}

	// Invalid credentials — re-render login page with error (401 like Python version)
	return h.renderPage(c, "login.html", map[string]interface{}{
		"Title": "Login — Drop",
		"Error": "Invalid username or password",
	}, http.StatusUnauthorized)
}

// logout clears the session cookie and redirects to login.
//
// Python equivalent: @router.get("/logout") async def logout(request):
func (h *Handler) logout(c echo.Context) error {
	// Delete the cookie by setting MaxAge to -1
	// This tells the browser to remove it immediately
	cookie := new(http.Cookie)
	cookie.Name = mw.CookieName
	cookie.Value = ""
	cookie.MaxAge = -1
	cookie.Path = "/"
	c.SetCookie(cookie)

	return c.Redirect(http.StatusFound, "/login")
}

// --- Dashboard ---

// dashboard renders the main dashboard with file list and stats.
// Redirects to /login if not authenticated.
//
// Python equivalent: @router.get("/") async def dashboard(request):
func (h *Handler) dashboard(c echo.Context) error {
	// Check admin auth — redirect to login if not authenticated
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.Redirect(http.StatusFound, "/login")
	}

	filesRepo := files.NewRepository(h.db)
	filesService := files.NewService(filesRepo)
	settingsRepo := settings.NewRepository(h.db)

	fileList := filesService.ListFiles()
	stats := filesService.GetStats()

	// Parse last_cleanup timestamp from settings (stored as RFC3339 string)
	var lastCleanup *time.Time
	raw := settingsRepo.GetAll()
	if val, ok := raw["last_cleanup"]; ok {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			lastCleanup = &t
		}
	}

	return h.renderPage(c, "dashboard.html", map[string]interface{}{
		"Title":       "Dashboard — Drop",
		"Files":       fileList,
		"Stats":       stats,
		"LastCleanup": lastCleanup,
	})
}

// --- Settings pages ---

// settingsPage renders the settings form.
//
// Python equivalent: @router.get("/settings") async def settings_page(request):
func (h *Handler) settingsPage(c echo.Context) error {
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.Redirect(http.StatusFound, "/login")
	}

	settingsRepo := settings.NewRepository(h.db)
	settingsService := settings.NewService(settingsRepo)
	current := settingsService.GetAll()

	return h.renderPage(c, "settings.html", map[string]interface{}{
		"Title":    "Settings — Drop",
		"Settings": current,
	})
}

// settingsSubmit handles the settings form POST.
// Reads form values, updates settings, and redirects back to the settings page.
//
// Python equivalent: @router.post("/settings") async def settings_submit(request, ...):
func (h *Handler) settingsSubmit(c echo.Context) error {
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.Redirect(http.StatusFound, "/login")
	}

	// Read all form fields. Unlike JSON APIs where we use *string for optional fields,
	// HTML forms always send all fields (as empty strings if not filled in).
	defaultExpiry := c.FormValue("default_expiry")
	maxExpiry := c.FormValue("max_expiry")
	maxFileSize := c.FormValue("max_file_size")
	storageLimit := c.FormValue("storage_limit")
	uploadAPIKey := c.FormValue("upload_api_key")

	// Build the update struct with pointers to the form values
	settingsRepo := settings.NewRepository(h.db)
	settingsService := settings.NewService(settingsRepo)
	settingsService.Update(settings.SettingsUpdate{
		DefaultExpiry: &defaultExpiry,
		MaxExpiry:     &maxExpiry,
		MaxFileSize:   &maxFileSize,
		StorageLimit:  &storageLimit,
		UploadAPIKey:  &uploadAPIKey,
	})

	// 303 redirect — POST/Redirect/GET pattern to prevent form resubmission
	return c.Redirect(http.StatusSeeOther, "/settings")
}

// --- HTMX endpoints ---
// These return HTML fragments that HTMX swaps into the page without a full reload.

// htmxDeleteFile deletes a file and returns an OOB stats update.
// When HTMX swaps the response, it:
//  1. Replaces the file row with nothing (empty response body means the row disappears)
//  2. Updates the stats via hx-swap-oob — a "piggyback" update that targets a different element
//
// Python equivalent: @router.delete("/api/files/{code}/htmx") async def htmx_delete_file(request, code):
func (h *Handler) htmxDeleteFile(c echo.Context) error {
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.HTML(http.StatusUnauthorized, "Unauthorized")
	}

	code := c.Param("code")

	// Delete the file (DB + disk)
	filesRepo := files.NewRepository(h.db)
	filesService := files.NewService(filesRepo)
	filesService.DeleteFile(code)

	// Re-render just the stats partial with OOB swap.
	// hx-swap-oob="innerHTML" tells HTMX to also update #stats-container
	// even though it's not the main hx-target.
	stats := filesService.GetStats()

	// Render the _stats.html partial into a buffer.
	// We must clone the template set before executing — html/template marks
	// templates as "executed" after use, and executed templates can't be cloned.
	// Since renderPage also clones, we need to keep h.tmpl pristine.
	t, err := h.tmpl.Clone()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template error")
	}
	var buf bytes.Buffer
	err = t.ExecuteTemplate(&buf, "stats", map[string]interface{}{
		"Stats": stats,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template error")
	}

	// Wrap the stats HTML in an OOB swap div
	html := fmt.Sprintf(`<div id="stats-container" hx-swap-oob="innerHTML">%s</div>`, buf.String())
	return c.HTML(http.StatusOK, html)
}

// htmxCleanup runs cleanup and returns the updated dashboard content.
// The HTMX button targets #dashboard-content with innerHTML swap,
// so we re-render the entire dashboard content partial.
//
// Python equivalent: @router.post("/cleanup") async def run_cleanup_action(request):
func (h *Handler) htmxCleanup(c echo.Context) error {
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.HTML(http.StatusUnauthorized, "Unauthorized")
	}

	// Run the cleanup (deletes expired, max-downloaded, orphaned)
	count := cleanup.RunCleanup(h.db, h.cfg)

	// Rebuild the dashboard content data
	filesRepo := files.NewRepository(h.db)
	filesService := files.NewService(filesRepo)
	settingsRepo := settings.NewRepository(h.db)

	fileList := filesService.ListFiles()
	stats := filesService.GetStats()

	var lastCleanup *time.Time
	raw := settingsRepo.GetAll()
	if val, ok := raw["last_cleanup"]; ok {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			lastCleanup = &t
		}
	}

	// Build cleanup message — "Cleaned up 3 files" or "Cleaned up 1 file"
	plural := "s"
	if count == 1 {
		plural = ""
	}
	cleanupMsg := fmt.Sprintf("Cleaned up %d file%s", count, plural)

	// Render _dashboard_content.html partial directly (not wrapped in base layout).
	// Clone first — same reason as htmxDeleteFile.
	t, err := h.tmpl.Clone()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template error")
	}
	var buf bytes.Buffer
	err = t.ExecuteTemplate(&buf, "dashboard_content", map[string]interface{}{
		"Files":          fileList,
		"Stats":          stats,
		"LastCleanup":    lastCleanup,
		"CleanupMessage": cleanupMsg,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template error")
	}

	return c.HTML(http.StatusOK, buf.String())
}

// manualUpload handles file uploads from the dashboard's upload modal.
// Unlike the PUT upload endpoint (streaming body), this handles multipart form data.
//
// Python equivalent: @router.post("/upload") async def manual_upload(request, file, expiry, max_downloads):
func (h *Handler) manualUpload(c echo.Context) error {
	if h.cfg.AdminEnabled && !mw.IsAdmin(c, h.cfg) {
		return c.HTML(http.StatusUnauthorized, "Unauthorized")
	}

	// Get the uploaded file from the multipart form.
	// c.FormFile() is like FastAPI's UploadFile = File(...)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No file provided")
	}

	// Open the uploaded file for reading
	src, err := fileHeader.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read upload")
	}
	defer src.Close()

	// Read form fields for expiry and max downloads
	expiry := c.FormValue("expiry")
	maxDownloadsStr := c.FormValue("max_downloads")

	// Generate a unique code for this upload
	code := upload.GenerateCode(h.db, 6)
	filename := fileHeader.Filename

	// Create the code directory: <files_dir>/<code>/
	codeDir := filepath.Join(h.cfg.FilesDir, code)
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create directory")
	}

	// Write the file to disk
	finalPath := filepath.Join(codeDir, filename)
	dst, err := os.Create(finalPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create file")
	}
	defer dst.Close()

	// io.Copy streams from src to dst — like Python's shutil.copyfileobj()
	size, err := io.Copy(dst, src)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save file")
	}

	// Parse expiry — use form value or fall back to default from settings
	settingsRepo := settings.NewRepository(h.db)
	settingsService := settings.NewService(settingsRepo)
	currentSettings := settingsService.GetAll()

	var expiresAt *time.Time
	if expiry != "" {
		expiresAt = upload.ParseExpiry(expiry)
	} else {
		expiresAt = upload.ParseExpiry(currentSettings.DefaultExpiry)
	}

	// Parse max downloads
	var maxDownloads *int
	if maxDownloadsStr != "" {
		if val, err := strconv.Atoi(maxDownloadsStr); err == nil {
			maxDownloads = &val
		}
	}

	// Create the DB record
	filesRepo := files.NewRepository(h.db)
	filesRepo.Create(code, filename, finalPath, size, maxDownloads, expiresAt)

	// Redirect back to dashboard (POST/Redirect/GET pattern)
	return c.Redirect(http.StatusSeeOther, "/")
}
