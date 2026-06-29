package domain

import (
	"context"
	"time"
)

type DailyEntry struct {
	RoutineID   uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

type Repository interface {
	Save(ctx context.Context, r *Routine) error
	FindByID(ctx context.Context, id uint) (*Routine, error)
	FindAll(ctx context.Context) ([]*Routine, error)
	Delete(ctx context.Context, id uint) error
	AddLog(ctx context.Context, log *RoutineLog) error
	DailyHistory(ctx context.Context, day time.Time) ([]DailyEntry, error)
}
