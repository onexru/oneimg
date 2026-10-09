package database

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"time"
)

// Legacy settings rows are archived verbatim before normalizing old databases.
// A non-ID=1 canonical row retains every column, including false/zero values.
type settingsLegacyArchive struct {
	OriginalID int    `gorm:"primaryKey;autoIncrement:false"`
	RowJSON    string `gorm:"type:text;not null"`
	ArchivedAt time.Time
}

func (settingsLegacyArchive) TableName() string { return "settings_legacy_archive" }

// EnsureSettingsSingleton preserves the row previously selected by First(),
// archives additional legacy rows and enforces ID=1 for future writes.
func EnsureSettingsSingleton(db *gorm.DB) error {
	// MySQL DDL implicitly commits. Create archival storage before changing any
	// legacy row, then do all row normalization in a separate real transaction.
	var needsArchive int64
	if err := db.Table("settings").Count(&needsArchive).Error; err != nil {
		return err
	}
	var idOneCount int64
	if err := db.Table("settings").Where("id = ?", 1).Count(&idOneCount).Error; err != nil {
		return err
	}
	if needsArchive > 1 || (needsArchive == 1 && idOneCount == 0) {
		if err := db.AutoMigrate(&settingsLegacyArchive{}); err != nil {
			return err
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("LOCK TABLE settings IN EXCLUSIVE MODE").Error; err != nil {
				return err
			}
		}
		var rows []map[string]interface{}
		if err := tx.Table("settings").Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			var canonicalID int
			if err := tx.Table("settings").Select("id").Order("id ASC").Limit(1).Scan(&canonicalID).Error; err != nil {
				return err
			}
			var hasCanonical int64
			if err := tx.Table("settings").Where("id = ?", 1).Count(&hasCanonical).Error; err != nil {
				return err
			}
			if hasCanonical > 0 {
				canonicalID = 1
			}
			if canonicalID != 1 || len(rows) > 1 {
				for _, row := range rows {
					encoded, err := json.Marshal(row)
					if err != nil {
						return err
					}
					var id int
					switch v := row["id"].(type) {
					case []byte:
						parsed, e := strconv.Atoi(string(v))
						if e != nil {
							return e
						}
						id = parsed
					case string:
						parsed, e := strconv.Atoi(v)
						if e != nil {
							return e
						}
						id = parsed
					case int64:
						id = int(v)
					case int:
						id = v
					default:
						return fmt.Errorf("unsupported settings ID type %T", v)
					}
					if err := tx.Create(&settingsLegacyArchive{OriginalID: id, RowJSON: string(encoded), ArchivedAt: time.Now()}).Error; err != nil {
						return err
					}
				}
				if err := tx.Table("settings").Where("id <> ?", canonicalID).Delete(nil).Error; err != nil {
					return err
				}
				if canonicalID != 1 {
					if err := tx.Exec("UPDATE settings SET id = 1 WHERE id = ?", canonicalID).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// Startup is single-app: install the invariant before accepting requests.
	return db.Transaction(func(tx *gorm.DB) error {
		switch tx.Dialector.Name() {
		case "sqlite":
			for _, event := range []string{"INSERT", "UPDATE"} {
				sql := fmt.Sprintf("CREATE TRIGGER IF NOT EXISTS settings_singleton_%s BEFORE %s ON settings WHEN NEW.id IS NULL OR NEW.id <> 1 BEGIN SELECT RAISE(ABORT, 'settings ID must be 1'); END", event, event)
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
			}
		default:
			if !tx.Migrator().HasConstraint("settings", "settings_singleton_id") {
				if err := tx.Exec("ALTER TABLE settings ADD CONSTRAINT settings_singleton_id CHECK (id = 1)").Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

type queryIndex struct {
	table, name string
	columns     []string
	optional    bool
}

// Separate URL/thumbnail B-trees let PostgreSQL BitmapOr serve the OR lookup.
// A single (url,thumbnail) index would not accelerate thumbnail-only matches.
func EnsureQueryIndexes(db *gorm.DB) error {
	indexes := []queryIndex{
		{"images", "idx_images_url", []string{"url"}, false},
		{"images", "idx_images_thumbnail", []string{"thumbnail"}, false},
		{"images", "idx_images_user_created", []string{"user_id", "created_at"}, false},
		{"images", "idx_images_uuid", []string{"uuid"}, false},
		{"images", "idx_images_bucket_id", []string{"bucket_id"}, false},
		{"images", "idx_images_created_at", []string{"created_at"}, false},
		{"images", "idx_images_md5", []string{"md5"}, false},
		{"images", "idx_images_tourist_fingerprint", []string{"tourist_fingerprint"}, true},
		{"images", "idx_images_uploaded_at", []string{"uploaded_at"}, true},
		// Existing PK(image_id,tag_id) covers forward joins; add reverse direction.
		{"image_to_tags", "idx_image_to_tags_tag_image", []string{"tag_id", "image_id"}, false},
	}
	for _, index := range indexes {
		if !db.Migrator().HasTable(index.table) {
			continue
		}
		missing := false
		for _, col := range index.columns {
			if !db.Migrator().HasColumn(index.table, col) {
				missing = true
			}
		}
		if missing {
			if index.optional {
				continue
			}
			return fmt.Errorf("index %s requires missing column", index.name)
		}
		if db.Migrator().HasIndex(index.table, index.name) {
			continue
		}
		stmt := &gorm.Statement{DB: db}
		quote := func(s string) string { return stmt.Quote(s) }
		columns := ""
		for i, col := range index.columns {
			if i > 0 {
				columns += ", "
			}
			columns += quote(col)
			// MySQL TEXT cannot be indexed without a prefix. 191 utf8mb4 chars
			// avoids old InnoDB key limits; equality is still verified by WHERE.
			if db.Dialector.Name() == "mysql" && (col == "url" || col == "thumbnail" || col == "uuid" || col == "md5" || col == "tourist_fingerprint") {
				columns += "(191)"
			}
		}
		if err := db.Exec("CREATE INDEX " + quote(index.name) + " ON " + quote(index.table) + " (" + columns + ")").Error; err != nil {
			return fmt.Errorf("create %s: %w", index.name, err)
		}
	}
	return nil
}

// AdvanceBucketSequence handles an explicit legacy/local ID=1 insertion. Never
// lowers an existing sequence. Only the startup transaction calls this helper.
func AdvanceBucketSequence(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	return tx.Exec(`SELECT setval(pg_get_serial_sequence('buckets','id'), GREATEST(COALESCE((SELECT MAX(id) FROM buckets),1), (SELECT last_value FROM buckets_id_seq)), true)`).Error
}
