package models

import (
	"time"
)

type Routine struct {
	Base
	Name        string `gorm:"not null" json:"name"`
	Description string `gorm:"type:text;" json:"description"`
}

type RoutineLog struct {
	Base
	RoutineID   uint      `gorm:"not null;index" json:"routine_id"`
	Routine     Routine   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CompletedAt time.Time `gorm:"autoCreateTime" json:"completed_at"`
}
