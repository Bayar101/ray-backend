package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/app"
	"github.com/Bayar101/ray-backend/internal/finance/domain"
)

type fakeRepo struct {
	transactions map[uint]*domain.Transaction
	seq          uint
	saveErr      error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{transactions: map[uint]*domain.Transaction{}}
}

type fakeCategoryRepo struct {
	categories map[uint]*domain.TransactionCategory
	seq        uint
	saveErr    error
}

func newFakeCategoryRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{categories: map[uint]*domain.TransactionCategory{}}
}

// Transaction Repository
func (f *fakeRepo) Save(_ context.Context, t *domain.Transaction) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	id := t.ID()
	if id == 0 {
		f.seq++
		id = f.seq
	}
	stored := domain.HydrateTransaction(id, t.TransactionCategoryID(), t.Amount(), t.Type(), t.Note(), t.Date())
	f.transactions[id] = stored
	*t = *stored
	return nil
}

func (f *fakeRepo) SaveMany(_ context.Context, ts []*domain.Transaction) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	for i, t := range ts {
		id := t.ID()
		if id == 0 {
			f.seq++
			id = f.seq
		}
		stored := domain.HydrateTransaction(id, t.TransactionCategoryID(), t.Amount(), t.Type(), t.Note(), t.Date())
		f.transactions[id] = stored
		*ts[i] = *stored
	}
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id uint) (*domain.Transaction, error) {
	if t, ok := f.transactions[id]; ok {
		return t, nil
	}
	return nil, domain.ErrTransactionNotFound
}

func (f *fakeRepo) FindAll(_ context.Context) ([]*domain.Transaction, error) {
	out := make([]*domain.Transaction, 0, len(f.transactions))
	for _, t := range f.transactions {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, id uint) error {
	if _, ok := f.transactions[id]; !ok {
		return domain.ErrTransactionNotFound
	}
	delete(f.transactions, id)
	return nil
}

// Category Repository
func (f *fakeCategoryRepo) Save(_ context.Context, c *domain.TransactionCategory) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	id := c.ID()
	if id == 0 {
		f.seq++
		id = f.seq
	}
	stored := domain.HydrateTransactionCategory(id, c.Name())
	f.categories[id] = stored
	*c = *stored
	return nil
}

func (f *fakeCategoryRepo) FindByID(_ context.Context, id uint) (*domain.TransactionCategory, error) {
	if c, ok := f.categories[id]; ok {
		return c, nil
	}
	return nil, domain.ErrTransactionCategoryNotFound
}

func (f *fakeCategoryRepo) FindAll(_ context.Context) ([]*domain.TransactionCategory, error) {
	out := make([]*domain.TransactionCategory, 0, len(f.categories))
	for _, c := range f.categories {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeCategoryRepo) Delete(_ context.Context, id uint) error {
	if _, ok := f.categories[id]; !ok {
		return domain.ErrTransactionCategoryNotFound
	}
	delete(f.categories, id)
	return nil
}

// Transaction Tests
func TestCreate_PersistsAndAssignsID(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	tx, err := svc.Create(context.Background(), 1, 100, domain.Income, "test", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.ID() == 0 {
		t.Fatalf("expected a generated id, got 0")
	}
}

func TestCreate_RejectsInvalidCategoryID(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.Create(context.Background(), 0, 100, domain.Income, "test", time.Now()); !errors.Is(err, domain.ErrTransactionCategoryRequired) {
		t.Fatalf("want ErrTransactionCategoryRequired, got %v", err)
	}
}

func TestCreate_RejectsInvalidAmount(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.Create(context.Background(), 1, 0, domain.Income, "test", time.Now()); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
}

func TestCreate_RejectsInvalidType(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.Create(context.Background(), 1, 100, "invalid", "test", time.Now()); !errors.Is(err, domain.ErrInvalidType) {
		t.Fatalf("want ErrInvalidType, got %v", err)
	}
}

func TestCreate_RejectsInvalidNote(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.Create(context.Background(), 1, 100, domain.Income, strings.Repeat("a", 1001), time.Now()); !errors.Is(err, domain.ErrNoteTooLong) {
		t.Fatalf("want ErrNoteTooLong, got %v", err)
	}
}

func TestCreate_RejectsInvalidDate(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.Create(context.Background(), 1, 100, domain.Income, "test", time.Time{}); !errors.Is(err, domain.ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
}

func TestBulkCreate_PersistsAllOrNothing(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	txs := make([]*domain.Transaction, 0, 2)
	for i := 0; i < 2; i++ {
		tx, err := domain.NewTransaction(1, 100, domain.Income, "test", time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		txs = append(txs, tx)
	}
	created, err := svc.BulkCreate(context.Background(), txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != len(txs) {
		t.Fatalf("want %d created, got %d", len(txs), len(created))
	}
	for i, tx := range created {
		if tx.ID() == 0 {
			t.Fatalf("expected a generated id, got 0")
		}
		if tx.TransactionCategoryID() != txs[i].TransactionCategoryID() {
			t.Fatalf("want category ID %d, got %d", txs[i].TransactionCategoryID(), tx.TransactionCategoryID())
		}
		if tx.Amount() != txs[i].Amount() {
			t.Fatalf("want amount %d, got %d", txs[i].Amount(), tx.Amount())
		}
		if tx.Type() != txs[i].Type() {
			t.Fatalf("want type %s, got %s", txs[i].Type(), tx.Type())
		}
		if tx.Note() != txs[i].Note() {
			t.Fatalf("want note %s, got %s", txs[i].Note(), tx.Note())
		}
		if tx.Date() != txs[i].Date() {
			t.Fatalf("want date %s, got %s", txs[i].Date(), tx.Date())
		}
	}
	all, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != len(txs) {
		t.Fatalf("want %d transactions, got %d", len(txs), len(all))
	}
	for i, tx := range all {
		if tx.ID() == 0 {
			t.Fatalf("expected a generated id, got 0")
		}
		if tx.TransactionCategoryID() != txs[i].TransactionCategoryID() {
			t.Fatalf("want category ID %d, got %d", txs[i].TransactionCategoryID(), tx.TransactionCategoryID())
		}
		if tx.Amount() != txs[i].Amount() {
			t.Fatalf("want amount %d, got %d", txs[i].Amount(), tx.Amount())
		}
		if tx.Type() != txs[i].Type() {
			t.Fatalf("want type %s, got %s", txs[i].Type(), tx.Type())
		}
		if tx.Note() != txs[i].Note() {
			t.Fatalf("want note %s, got %s", txs[i].Note(), tx.Note())
		}
		if tx.Date() != txs[i].Date() {
			t.Fatalf("want date %s, got %s", txs[i].Date(), tx.Date())
		}
	}
}

// Category Tests
func TestCreateCategory_PersistsAndAssignsID(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	cat, err := svc.CreateCategory(context.Background(), "test category")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat.ID() == 0 {
		t.Fatalf("expected a generated id, got 0")
	}
}

func TestCreateCategory_RejectsEmptyName(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	if _, err := svc.CreateCategory(context.Background(), ""); !errors.Is(err, domain.ErrTransactionCategoryNameRequired) {
		t.Fatalf("want ErrTransactionCategoryNameRequired, got %v", err)
	}
}

func TestListCategories_ReturnsAll(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo())
	_, _ = svc.CreateCategory(context.Background(), "test category 1")
	_, _ = svc.CreateCategory(context.Background(), "test category 2")
	cats, err := svc.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("want 2 categories, got %d", len(cats))
	}
}
