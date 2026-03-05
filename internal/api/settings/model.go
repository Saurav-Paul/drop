// Package settings handles server configuration stored in the database.
// This file defines the GORM model — the Go equivalent of settings_model.py.
package settings

// Setting is the GORM model for the "settings" table.
// It's a simple key-value store used for server configuration
// (default_expiry, max_file_size, storage_limit, etc.).
//
// In Go, we define database models as structs with "gorm" tags.
// Tags are metadata attached to struct fields — similar to SQLAlchemy's Column() definitions.
//
// Python equivalent:
//
//	class SettingModel(Base):
//	    __tablename__ = "settings"
//	    key = Column(String, primary_key=True)
//	    value = Column(String, nullable=False, default="")
type Setting struct {
	Key   string `gorm:"primaryKey;column:key"`              // The setting name (e.g. "default_expiry")
	Value string `gorm:"column:value;not null;default:''"` // The setting value (e.g. "24h")
}

// TableName tells GORM which database table this struct maps to.
// Without this, GORM would auto-generate the name as "settings" (lowercase plural),
// which happens to match, but being explicit is clearer.
func (Setting) TableName() string {
	return "settings"
}
