package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Bayar101/ray-backend/internal/models"
	"gorm.io/gorm"
)

type RoutineService struct {
	db *gorm.DB
}

type DailyEntry struct {
	models.Routine
	Completed bool `json:"completed"`
}

func NewRoutineService(db *gorm.DB) *RoutineService {
	return &RoutineService{db: db}
}

func (s *RoutineService) Create(ctx context.Context, name, description string) (models.Routine, error) {
	r := models.Routine{
		Name:        name,
		Description: description,
	}
	if name == "" {
		return models.Routine{}, fmt.Errorf("name is required")
	}
	if err := s.db.WithContext(ctx).Create(&r).Error; err != nil {
		return models.Routine{}, fmt.Errorf("failed to create routine: %w", err)
	}
	return r, nil
}

func (s *RoutineService) List(ctx context.Context) ([]models.Routine, error) {
	var routines []models.Routine
	if err := s.db.WithContext(ctx).Find(&routines).Error; err != nil {
		return nil, fmt.Errorf("failed to get routines: %w", err)
	}
	return routines, nil
}

func (s *RoutineService) Get(ctx context.Context, id uint) (models.Routine, error) {
	var routine models.Routine
	if err := s.db.WithContext(ctx).First(&routine, id).Error; err != nil {
		return models.Routine{}, fmt.Errorf("failed to get routine: %w", err)
	}
	return routine, nil
}

func (s *RoutineService) Complete(ctx context.Context, id uint) (models.RoutineLog, error) {
	var r models.Routine
	if err := s.db.WithContext(ctx).First(&r, id).Error; err != nil {
		return models.RoutineLog{}, fmt.Errorf("failed to get routine: %w", err)
	}
	log := models.RoutineLog{
		RoutineID:   id,
		CompletedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		return models.RoutineLog{}, fmt.Errorf("failed to complete routine: %w", err)
	}
	return log, nil
}

// func (s *RoutineService) DailyHistory(ctx context.Context, date time.Time) ([]DailyEntry, error) {
// 	var routines []models.Routine
// 	res := s.db.WithContext(ctx).Find(&routines)
// 	if res.Error != nil {
// 		return nil, res.Error
// 	}

// 	var entries []DailyEntry
// 	for _, routine := range routines {
// 		count := int64(0)
// 		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
// 		endOfDay := startOfDay.Add(24 * time.Hour)
// 		res := s.db.WithContext(ctx).
// 			Model(&models.RoutineLog{}).
// 			Where("routine_id = ? AND completed_at BETWEEN ? AND ?", routine.ID, startOfDay, endOfDay).
// 			Count(&count)

// 		if res.Error != nil {
// 			return nil, res.Error
// 		}
// 		entries = append(entries, DailyEntry{
// 			Routine:   routine,
// 			Completed: count > 0,
// 		})
// 	}

// 	return entries, nil
// }

func (s *RoutineService) DailyHistory(ctx context.Context, date time.Time) ([]DailyEntry, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var entries []DailyEntry
	err := s.db.WithContext(ctx).
		Model(&models.Routine{}).
		Select("routines.*, COUNT(routine_logs.id) > 0 AS completed").
		Joins(`LEFT JOIN routine_logs 
			ON routine_logs.routine_id = routines.id 
			AND routine_logs.completed_at >= ? 
			AND routine_logs.completed_at < ? 
			AND routine_logs.deleted_at IS NULL`, startOfDay, endOfDay).
		Group("routines.id").
		Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get daily history: %w", err)
	}

	return entries, nil
}
