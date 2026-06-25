package routes

import (
	"testing"

	"github.com/Bayar101/ray-backend/internal/models"
	"github.com/Bayar101/ray-backend/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
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
			svc := services.NewRoutineService(newTestDB(t))
			_, err := svc.Create(t.Context(), tt.input, "")

			if tt.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}
