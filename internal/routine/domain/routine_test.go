package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Bayar101/ray-backend/internal/routine/domain"
)

func TestNewRoutine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "Morning run", false},
		{"empty name", "", true},
		{"name too long", strings.Repeat("a", 101), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewRoutine(tt.input, "")
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

func TestRoutine_Rename_RejectsEmpty(t *testing.T) {
	r, _ := domain.NewRoutine("Morning run", "")
	if err := r.Rename(""); !errors.Is(err, domain.ErrNameRequired) {
		t.Fatalf("want ErrNameRequired, got %v", err)
	}
	if r.Name() != "Morning run" {
		t.Fatalf("name mutated on failed rename: %q", r.Name())
	}
}

func TestRoutine_Describe_RejectsTooLong(t *testing.T) {
	r, _ := domain.NewRoutine("Morning run", "")
	if err := r.Describe(strings.Repeat("a", 1001)); !errors.Is(err, domain.ErrDescriptionTooLong) {
		t.Fatalf("want ErrDescriptionTooLong, got %v", err)
	}
	if r.Description() != "" {
		t.Fatalf("description mutated on failed describe: %q", r.Description())
	}
}
