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
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}
