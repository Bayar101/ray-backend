package app

import (
	"context"
	"time"

	"github.com/Bayar101/ray-backend/internal/routine/domain"
)

type Service struct {
	r domain.Repository
}

func NewService(r domain.Repository) *Service {
	return &Service{r: r}
}

func (s *Service) Create(ctx context.Context, name, description string) (*domain.Routine, error) {
	r, err := domain.NewRoutine(name, description)
	if err != nil {
		return nil, err
	}
	if err := s.r.Save(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Routine, error) {
	return s.r.FindAll(ctx)
}

func (s *Service) Get(ctx context.Context, id uint) (*domain.Routine, error) {
	return s.r.FindByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uint, name, description string) (*domain.Routine, error) {
	r, err := s.r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		if err := r.Rename(name); err != nil {
			return nil, err
		}
	}
	if description != "" {
		r.Describe(description)
	}
	if err := s.r.Save(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.r.Delete(ctx, id)
}

func (s *Service) Complete(ctx context.Context, id uint) (*domain.RoutineLog, error) {
	r, err := s.r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	log := domain.LogCompletion(r)
	if err := s.r.AddLog(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *Service) DailyHistory(ctx context.Context, day time.Time) ([]domain.DailyEntry, error) {
	return s.r.DailyHistory(ctx, day)
}
