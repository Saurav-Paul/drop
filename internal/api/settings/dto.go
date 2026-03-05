package settings

// This file defines the request/response structs (DTOs) for the settings API.
// Go equivalent of backend/api/settings/dto/settings.py.

// SettingsResponse is the JSON shape returned by GET /api/settings.
// The `json` tags control the JSON field names — like Pydantic's field names.
//
// Python equivalent:
//
//	class SettingsResponse(BaseModel):
//	    default_expiry: str
//	    max_expiry: str
//	    ...
type SettingsResponse struct {
	DefaultExpiry string `json:"default_expiry"`
	MaxExpiry     string `json:"max_expiry"`
	MaxFileSize   string `json:"max_file_size"`
	StorageLimit  string `json:"storage_limit"`
	UploadAPIKey  string `json:"upload_api_key"`
}

// SettingsUpdate is the JSON body for PUT /api/settings.
// Fields are *string (pointer to string) so we can distinguish between
// "field not sent" (nil) and "field sent as empty string" ("").
// This enables partial updates — only update fields that are present in the request.
//
// Python equivalent:
//
//	class SettingsUpdate(BaseModel):
//	    default_expiry: str | None = None
//	    max_expiry: str | None = None
//	    ...
type SettingsUpdate struct {
	DefaultExpiry *string `json:"default_expiry,omitempty"` // omitempty: skip in JSON output if nil
	MaxExpiry     *string `json:"max_expiry,omitempty"`
	MaxFileSize   *string `json:"max_file_size,omitempty"`
	StorageLimit  *string `json:"storage_limit,omitempty"`
	UploadAPIKey  *string `json:"upload_api_key,omitempty"`
}
