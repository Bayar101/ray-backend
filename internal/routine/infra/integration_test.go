//go:build integration

package infra_test

import (
	"context"
	"testing"
	"time"

	"github.com/Bayar101/ray-backend/internal/routine/domain"
	"github.com/Bayar101/ray-backend/internal/routine/infra"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newPG(t *testing.T) *gorm.DB {
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(), // wait for listening port + 2nd "ready" log (init restart)
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	db, err := gorm.Open(pgdriver.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(infra.Models()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDailyHistory_CompletedFlag(t *testing.T) {
	db := newPG(t)
	repo := infra.NewGormRepository(db)
	ctx := context.Background()

	done, _ := domain.NewRoutine("done", "")
	_ = repo.Save(ctx, done)
	_, _ = domain.NewRoutine("skipped", "")
	skipped, _ := domain.NewRoutine("skipped", "")
	_ = repo.Save(ctx, skipped)

	_ = repo.AddLog(ctx, domain.LogCompletion(done))

	entries, err := repo.DailyHistory(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name] = e.Completed
	}
	if !got["done"] {
		t.Error("done should be completed")
	}
	if got["skipped"] {
		t.Error("skipped should not be completed")
	}
}
