//go:build integration

package infra_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/domain"
	"github.com/Bayar101/ray-backend/internal/finance/infra"
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

func TestCategory_DuplicateName(t *testing.T) {
	db := newPG(t)
	repo := infra.NewTransactionCategoryGormRepository(db)
	ctx := context.Background()

	c1, _ := domain.NewTransactionCategory("Food")
	if err := repo.Save(ctx, c1); err != nil {
		t.Fatal(err)
	}
	c2, _ := domain.NewTransactionCategory("Food")
	if err := repo.Save(ctx, c2); !errors.Is(err, domain.ErrDuplicateTransactionCategory) {
		t.Fatalf("want ErrDuplicateTransactionCategory, got %v", err)
	}
}

func TestSaveMany_RollBackOnFailure(t *testing.T) {
	db := newPG(t)
	repo := infra.NewTransactionGormRepository(db)
	ctx := context.Background()

	// Both rows carry the SAME explicit primary key, so the single batch INSERT
	// violates the PK constraint — forcing the whole transaction to roll back.
	good := domain.HydrateTransaction(1, 1, 100, domain.Income, "", time.Now())
	bad := domain.HydrateTransaction(1, 1, 200, domain.Expense, "", time.Now())

	if err := repo.SaveMany(ctx, []*domain.Transaction{good, bad}); err == nil {
		t.Fatal("expected batch to fail on duplicate primary key")
	}
	all, _ := repo.FindAll(ctx)
	if len(all) != 0 {
		t.Fatalf("rollback failed: %d rows persisted", len(all))
	}
}
