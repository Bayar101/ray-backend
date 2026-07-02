package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/domain"
)

type createTestInput struct {
	categoryID uint
	amount     int64
	txType     domain.TransactionType
	note       string
	date       time.Time
}

func TestNewTransaction(t *testing.T) {
	tests := []struct {
		name    string
		input   createTestInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: createTestInput{
				categoryID: 1,
				amount:     100,
				txType:     domain.Income,
				note:       "test note",
				date:       time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty category ID",
			input: createTestInput{
				categoryID: 0,
				amount:     100,
				txType:     domain.Income,
				note:       "test note",
				date:       time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid amount",
			input: createTestInput{
				categoryID: 1,
				amount:     -100,
				txType:     domain.Income,
				note:       "test note",
				date:       time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			input: createTestInput{
				categoryID: 1,
				amount:     100,
				txType:     "invalid",
				note:       "test note",
				date:       time.Now(),
			},
			wantErr: true,
		},
		{
			name: "note too long",
			input: createTestInput{
				categoryID: 1,
				amount:     100,
				txType:     domain.Income,
				note:       strings.Repeat("a", 1001),
				date:       time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewTransaction(tt.input.categoryID, tt.input.amount, tt.input.txType, tt.input.note, tt.input.date)
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

func TestTransaction_SetCategoryID_RejectsZero(t *testing.T) {
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", time.Now())
	if err := tx.SetCategoryID(0); !errors.Is(err, domain.ErrTransactionCategoryRequired) {
		t.Fatalf("want ErrTransactionCategoryRequired, got %v", err)
	}
	if tx.CategoryID() != 1 {
		t.Fatalf("categoryID mutated on failed set: %d", tx.CategoryID())
	}
}

func TestTransaction_SetAmount_RejectsNegative(t *testing.T) {
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", time.Now())
	if err := tx.SetAmount(-100); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
	if tx.Amount() != 100 {
		t.Fatalf("amount mutated on failed set: %d", tx.Amount())
	}
}

func TestTransaction_SetType_RejectsInvalid(t *testing.T) {
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", time.Now())
	if err := tx.SetType("invalid"); !errors.Is(err, domain.ErrInvalidType) {
		t.Fatalf("want ErrInvalidType, got %v", err)
	}
	if tx.Type() != domain.Income {
		t.Fatalf("type mutated on failed set: %s", tx.Type())
	}
}

func TestTransaction_SetNote_RejectsTooLong(t *testing.T) {
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", time.Now())
	if err := tx.SetNote(strings.Repeat("a", 1001)); !errors.Is(err, domain.ErrNoteTooLong) {
		t.Fatalf("want ErrNoteTooLong, got %v", err)
	}
	if tx.Note() != "test note" {
		t.Fatalf("note mutated on failed set: %s", tx.Note())
	}
}

func TestTransaction_SetDate_RejectsZero(t *testing.T) {
	original := time.Now()
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", original)
	if err := tx.SetDate(time.Time{}); !errors.Is(err, domain.ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
	if !tx.Date().Equal(original) {
		t.Fatalf("date mutated on failed set: %v", tx.Date())
	}
}
