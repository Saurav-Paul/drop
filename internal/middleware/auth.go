// Package middleware provides Echo middleware for the application.
// This file handles admin authentication — the Go equivalent of backend/auth.py.
package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Saurav-Paul/drop/internal/config"
)

// CookieName is the name of the session cookie — must match the Python version
// so existing browser sessions stay valid after switching from Python to Go.
const CookieName = "drop_session"

// MakeToken creates an HMAC-SHA256 token from the admin credentials.
// This MUST produce the exact same hex string as the Python version:
//
//	hmac.new(f"{user}:{pass}".encode(), b"drop_session", hashlib.sha256).hexdigest()
//
// If the output differs, existing browser cookies from the Python server won't work.
func MakeToken(user, pass string) string {
	// Create HMAC using SHA-256 with "user:pass" as the key
	// In Python: hmac.new(key, message, hashlib.sha256)
	// In Go: hmac.New(hash_func, key) then Write(message)
	key := []byte(user + ":" + pass)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("drop_session"))

	// hex.EncodeToString is like Python's .hexdigest()
	return hex.EncodeToString(mac.Sum(nil))
}

// IsAdmin checks if the request has valid admin credentials.
// Checks two methods (same order as Python):
//  1. Header auth: X-Admin-User + X-Admin-Pass (for API/curl)
//  2. Cookie auth: drop_session cookie with valid HMAC token (for browser)
//
// Python equivalent:
//
//	def is_admin(request: Request) -> bool:
//	    if not ADMIN_ENABLED: return True
//	    # check headers...
//	    # check cookie...
func IsAdmin(c echo.Context, cfg *config.Config) bool {
	// If admin is not configured (no user/pass set), allow everything
	// Same as Python's: if not ADMIN_ENABLED: return True
	if !cfg.AdminEnabled {
		return true
	}

	// Check header-based auth (API / curl requests)
	user := c.Request().Header.Get("X-Admin-User")
	pass := c.Request().Header.Get("X-Admin-Pass")
	if user != "" && pass != "" {
		// subtle.ConstantTimeCompare prevents timing attacks — same as Python's secrets.compare_digest()
		// It returns 1 if equal, 0 if not (unlike == which returns bool)
		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.AdminUser)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.AdminPass)) == 1
		return userMatch && passMatch
	}

	// Check cookie-based auth (browser sessions)
	cookie, err := c.Cookie(CookieName)
	if err != nil {
		// No cookie found — not authenticated
		return false
	}

	// Verify the cookie token matches the expected HMAC
	expected := MakeToken(cfg.AdminUser, cfg.AdminPass)
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) == 1
}

// RequireAdmin returns an Echo middleware that blocks non-admin requests.
// Attach it to route groups that need admin access.
//
// Usage in handler registration:
//
//	g.Use(middleware.RequireAdmin(cfg))
//
// Python equivalent:
//
//	if not is_admin(request):
//	    raise HTTPException(status_code=401, detail="Admin access required")
func RequireAdmin(cfg *config.Config) echo.MiddlewareFunc {
	// This returns a function that returns a function — it's the middleware pattern in Echo:
	//   RequireAdmin(cfg) → middleware function → wraps the actual handler
	//
	// The outer function captures cfg (closure), the middle function is what Echo calls
	// for each request, and "next" is the actual route handler to call if auth passes.
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !IsAdmin(c, cfg) {
				return echo.NewHTTPError(http.StatusUnauthorized, "Admin access required")
			}
			// Auth passed — call the actual route handler
			return next(c)
		}
	}
}
