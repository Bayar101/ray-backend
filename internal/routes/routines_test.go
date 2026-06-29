package routes

import (
	"context"
	"testing"

	financeinfra "github.com/Bayar101/ray-backend/internal/finance/infra"
	routineapp "github.com/Bayar101/ray-backend/internal/routine/app"
	routineinfra "github.com/Bayar101/ray-backend/internal/routine/infra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(append(routineinfra.Models(), financeinfra.Models()...)...); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name    string // subTest label
		input   string // the routine name to create
		wantErr bool   // expected outcome
	}{
		{name: "valid name", input: "Morning run", wantErr: false},
		{name: "empty name", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := routineapp.NewService(routineinfra.NewGormRepository(newTestDB(t)))
			_, err := svc.Create(context.Background(), tt.input, "")

			if tt.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}
