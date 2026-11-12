package database

import (
	"context"
	"fmt"
	"io/fs"

	"gorm.io/gorm"
)

// Verify checks the exact migration stream without creating metadata or applying SQL.
func Verify(ctx context.Context, db *gorm.DB, files fs.FS, dir string) error {
	migrations, err := LoadMigrations(files, dir)
	if err != nil {
		return err
	}
	var records []AppliedRecord
	if err := db.WithContext(ctx).Table(migrationTable).Select("version,name,checksum,applied_at").Scan(&records).Error; err != nil {
		return fmt.Errorf("migration metadata unavailable")
	}
	if len(records) != len(migrations) {
		return fmt.Errorf("migration count mismatch")
	}
	applied := map[string]AppliedRecord{}
	for _, record := range records {
		applied[record.Version] = record
	}
	for _, migration := range migrations {
		record, ok := applied[migration.Version]
		if !ok || record.Name != migration.Name || record.Checksum != migration.Checksum {
			return fmt.Errorf("migration mismatch at version %s", migration.Version)
		}
	}
	return nil
}
