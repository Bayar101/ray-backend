package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Bayar101/ray-backend/internal/routine/app"
	"github.com/Bayar101/ray-backend/internal/routine/domain"
)

type fakeRepo struct {
	routines map[uint]*domain.Routine
	logs     []*domain.RoutineLog
	seq      uint
	saveErr  error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{routines: map[uint]*domain.Routine{}, logs: []*domain.RoutineLog{}}
}

func (f *fakeRepo) Save(_ context.Context, r *domain.Routine) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	id := r.ID()
	if id == 0 {
		f.seq++
		id = f.seq
	}
	stored := domain.Hydrate(id, r.Name(), r.Description())
	f.routines[id] = stored
	*r = *stored
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id uint) (*domain.Routine, error) {
	if r, ok := f.routines[id]; ok {
		return r, nil
	}
	return nil, domain.ErrRoutineNotFound
}

func (f *fakeRepo) FindAll(_ context.Context) ([]*domain.Routine, error) {
	out := make([]*domain.Routine, 0, len(f.routines))
	for _, r := range f.routines {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, id uint) error {
	if _, ok := f.routines[id]; !ok {
		return domain.ErrRoutineNotFound
	}
	delete(f.routines, id)
	return nil
}

func (f *fakeRepo) AddLog(_ context.Context, l *domain.RoutineLog) error {
	f.logs = append(f.logs, l)
	return nil
}

func (f *fakeRepo) DailyHistory(_ context.Context, _ time.Time) ([]domain.DailyEntry, error) {
	return nil, nil
}

func TestCreate_PersistsAndAssignsID(t *testing.T) {
	svc := app.NewService(newFakeRepo())
	r, err := svc.Create(context.Background(), "Morning run", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID() == 0 {
		t.Fatal("expected a generated id, got 0")
	}
}

func TestCreate_RejectsEmptyName(t *testing.T) {
	svc := app.NewService(newFakeRepo())
	if _, err := svc.Create(context.Background(), "", ""); !errors.Is(err, domain.ErrNameRequired) {
		t.Fatalf("want ErrNameRequired, got %v", err)
	}
}

func TestComplete_UnknownRoutine(t *testing.T) {
	svc := app.NewService(newFakeRepo())
	if _, err := svc.Complete(context.Background(), 999); !errors.Is(err, domain.ErrRoutineNotFound) {
		t.Fatalf("want ErrRoutineNotFound, got %v", err)
	}
}

func TestComplete_LogsExistingRoutine(t *testing.T) {
	repo := newFakeRepo()
	svc := app.NewService(repo)
	r, _ := svc.Create(context.Background(), "Morning run", "")

	log, err := svc.Complete(context.Background(), r.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.RoutineID() != r.ID() {
		t.Fatalf("log points at %d, want %d", log.RoutineID(), r.ID())
	}
	if len(repo.logs) != 1 {
		t.Fatalf("want 1 log persisted, got %d", len(repo.logs))
	}
}
