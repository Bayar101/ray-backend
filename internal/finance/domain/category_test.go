package domain_test

import (
	"errors"
	"testing"

	"github.com/Bayar101/ray-backend/internal/finance/domain"
)

func TestNewTransactionCategory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   "test category",
			wantErr: false,
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewTransactionCategory(tt.input)
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

func TestTransactionCategory_Rename_RejectsEmpty(t *testing.T) {
	c, _ := domain.NewTransactionCategory("test category")
	if err := c.Rename(""); !errors.Is(err, domain.ErrTransactionCategoryNameRequired) {
		t.Fatalf("want ErrTransactionCategoryNameRequired, got %v", err)
	}
	if c.Name() != "test category" {
		t.Fatalf("name mutated on failed rename: %q", c.Name())
	}
}
