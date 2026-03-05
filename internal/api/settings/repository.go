package settings

// This file is the data access layer — all database queries for settings.
// Go equivalent of backend/api/settings/repositories/settings_repository.py.

import "gorm.io/gorm"

// Repository provides database operations for the settings table.
// We use a struct with a db field instead of module-level functions,
// so the database connection is passed explicitly (no global state).
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository with the given database connection.
// This pattern is called "dependency injection" — we pass in what we need
// rather than importing a global. Makes testing easier too.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetAll returns all settings as a map.
// Go's map[string]string is like Python's dict[str, str].
//
// Python equivalent:
//
//	def get_all() -> dict[str, str]:
//	    settings = session.query(SettingModel).all()
//	    return {s.key: s.value for s in settings}
func (r *Repository) GetAll() map[string]string {
	var settings []Setting

	// Find() loads all rows into the slice (Go's version of a list)
	r.db.Find(&settings)

	// Build a map from the results — like a dict comprehension in Python
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result
}

// Set creates or updates a single setting (upsert).
// Uses GORM's "assign and find or create" pattern.
//
// Python equivalent:
//
//	def set(key, value):
//	    setting = session.query(SettingModel).filter_by(key=key).first()
//	    if setting:
//	        setting.value = value
//	    else:
//	        session.add(SettingModel(key=key, value=value))
//	    session.commit()
func (r *Repository) Set(key, value string) {
	var setting Setting

	// Try to find the existing row by key
	result := r.db.Where("key = ?", key).First(&setting)

	if result.Error != nil {
		// Row doesn't exist — create it
		// result.Error is gorm.ErrRecordNotFound when no row matches
		r.db.Create(&Setting{Key: key, Value: value})
	} else {
		// Row exists — update the value
		r.db.Model(&setting).Update("value", value)
	}
}

// SetMany creates or updates multiple settings in sequence.
//
// Python equivalent:
//
//	def set_many(data: dict[str, str]):
//	    for key, value in data.items():
//	        set(key, value)
func (r *Repository) SetMany(data map[string]string) {
	for key, value := range data {
		r.Set(key, value)
	}
}
