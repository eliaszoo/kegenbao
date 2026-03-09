package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"kegenbao/internal/config"
	"kegenbao/internal/models"
)

var DB *gorm.DB

func InitDB(cfg *config.DatabaseConfig) error {
	// Ensure data directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Enable WAL mode for better concurrency
	db, err := gorm.Open(sqlite.Open(cfg.Path+"?_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.FollowUpRecord{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	DB = db
	log.Println("Database initialized successfully")
	return nil
}

func GetDB() *gorm.DB {
	return DB
}