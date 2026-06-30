package infra

import (
	"context"
	"errors"
	"time"

	"github.com/Bayar101/ray-backend/internal/routine/domain"
	"gorm.io/gorm"
)

type routineRecord struct {
	ID          uint           `gorm:"primaryKey"`
	Name        string         `gorm:"not null;uniqueIndex"`
	Description string         `gorm:"type:text"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (routineRecord) TableName() string { return "routines" }

type routineLogRecord struct {
	ID          uint           `gorm:"primaryKey"`
	RoutineID   uint           `gorm:"not null;index"`
	CompletedAt time.Time      `gorm:"autoCreateTime"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (routineLogRecord) TableName() string { return "routine_logs" }

func Models() []any { return []any{&routineRecord{}, &routineLogRecord{}} }

func toDomain(rec routineRecord) *domain.Routine {
	return domain.Hydrate(rec.ID, rec.Name, rec.Description)
}

func toRecord(r *domain.Routine) routineRecord {
	return routineRecord{ID: r.ID(), Name: r.Name(), Description: r.Description()}
}

type GormRepository struct{ db *gorm.DB }

func NewGormRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) Save(ctx context.Context, ro *domain.Routine) error {
	rec := toRecord(ro)
	if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
		return err
	}
	*ro = *toDomain(rec)
	return nil
}

func (r *GormRepository) FindByID(ctx context.Context, id uint) (*domain.Routine, error) {
	var rec routineRecord
	if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRoutineNotFound
		}
		return nil, err
	}
	return toDomain(rec), nil
}

func (r *GormRepository) FindAll(ctx context.Context) ([]*domain.Routine, error) {
	var recs []routineRecord
	if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.Routine, len(recs))
	for i, rec := range recs {
		out[i] = toDomain(rec)
	}
	return out, nil
}

func (r *GormRepository) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&routineRecord{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoutineNotFound
	}
	return nil
}

func (r *GormRepository) AddLog(ctx context.Context, log *domain.RoutineLog) error {
	rec := routineLogRecord{RoutineID: log.RoutineID(), CompletedAt: log.CompletedAt()}
	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return err
	}
	*log = *domain.HydrateLog(rec.ID, rec.RoutineID, rec.CompletedAt)
	return nil
}

func (r *GormRepository) DailyHistory(ctx context.Context, day time.Time) ([]domain.DailyEntry, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	var entries []domain.DailyEntry
	err := r.db.WithContext(ctx).
		Model(&routineRecord{}).
		Select("routines.id AS routine_id, routines.name, routines.description, COUNT(routine_logs.id) > 0 AS completed").
		Joins(`LEFT JOIN routine_logs
			ON routine_logs.routine_id = routines.id
			AND routine_logs.completed_at >= ?
			AND routine_logs.completed_at < ?
			AND routine_logs.deleted_at IS NULL`, start, end).
		Group("routines.id").
		Scan(&entries).Error
	if err != nil {
		return nil, err
	}
	return entries, nil
}
