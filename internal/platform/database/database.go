package database

import (
	"time"

	"github.com/Bayar101/ray-backend/internal/platform/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.DB, models ...any) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()))
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(models...); err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(70)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}
