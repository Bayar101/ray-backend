package database

import (
	"time"

	"github.com/Bayar101/ray-backend/internal/config"
	"github.com/Bayar101/ray-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.DB) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()))
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(70)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}
