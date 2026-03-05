package settings

// This file contains the business logic for settings.
// Go equivalent of backend/api/settings/services/settings_service.py.

// settingsDefaults defines default values for all settings.
// If a setting doesn't exist in the database, these defaults are used.
// Same as SETTINGS_DEFAULTS in the Python version.
var settingsDefaults = map[string]string{
	"default_expiry": "24h",
	"max_expiry":     "7d",
	"max_file_size":  "100MB",
	"storage_limit":  "1GB",
	"upload_api_key": "",
}

// Service handles business logic for settings.
// It sits between the handler (HTTP layer) and the repository (database layer).
type Service struct {
	repo *Repository
}

// NewService creates a new Service with the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetAll returns all settings, filling in defaults for any missing values.
//
// Python equivalent:
//
//	def get_all() -> SettingsResponse:
//	    raw = settings_repository.get_all()
//	    data = {field: raw.get(field, default) for field, default in SETTINGS_DEFAULTS.items()}
//	    return SettingsResponse(**data)
func (s *Service) GetAll() SettingsResponse {
	// Fetch stored settings from the database
	raw := s.repo.GetAll()

	// Build response by merging DB values over defaults.
	// For each default key, use the DB value if it exists, otherwise use the default.
	merged := make(map[string]string, len(settingsDefaults))
	for key, defaultVal := range settingsDefaults {
		if val, exists := raw[key]; exists {
			merged[key] = val
		} else {
			merged[key] = defaultVal
		}
	}

	return SettingsResponse{
		DefaultExpiry: merged["default_expiry"],
		MaxExpiry:     merged["max_expiry"],
		MaxFileSize:   merged["max_file_size"],
		StorageLimit:  merged["storage_limit"],
		UploadAPIKey:  merged["upload_api_key"],
	}
}

// Update persists non-nil fields from the update request and returns all settings.
//
// Python equivalent:
//
//	def update(data: SettingsUpdate) -> SettingsResponse:
//	    updates = {k: v for k, v in data.model_dump().items() if v is not None}
//	    if updates:
//	        settings_repository.set_many(updates)
//	    return get_all()
func (s *Service) Update(data SettingsUpdate) SettingsResponse {
	// Collect non-nil fields into a map for the repository.
	// Each *string check is like Python's `if v is not None`.
	updates := make(map[string]string)
	if data.DefaultExpiry != nil {
		updates["default_expiry"] = *data.DefaultExpiry
	}
	if data.MaxExpiry != nil {
		updates["max_expiry"] = *data.MaxExpiry
	}
	if data.MaxFileSize != nil {
		updates["max_file_size"] = *data.MaxFileSize
	}
	if data.StorageLimit != nil {
		updates["storage_limit"] = *data.StorageLimit
	}
	if data.UploadAPIKey != nil {
		updates["upload_api_key"] = *data.UploadAPIKey
	}

	if len(updates) > 0 {
		s.repo.SetMany(updates)
	}

	return s.GetAll()
}
