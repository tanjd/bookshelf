// Package db initializes and returns the application database handle.
package db

import (
	"os"
	"path/filepath"

	"github.com/tanjd/bookshelf/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open initialises a SQLite database at the given path, creates the data
// directory if needed, runs AutoMigrate for all models, and returns the
// *gorm.DB handle.
func Open(dbPath string) (*gorm.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Book{},
		&models.Copy{},
		&models.LoanRequest{},
		&models.Notification{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
