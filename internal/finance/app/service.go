package app

import (
	"context"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/domain"
)

type Service struct {
	tx  domain.TransactionRepository
	cat domain.TransactionCategoryRepository
}

func NewService(tx domain.TransactionRepository, cat domain.TransactionCategoryRepository) *Service {
	return &Service{tx: tx, cat: cat}
}

func (s *Service) Create(ctx context.Context, categoryID uint, amount int64, txType domain.TransactionType, note string, date time.Time) (*domain.Transaction, error) {
	t, err := domain.NewTransaction(categoryID, amount, txType, note, date)
	if err != nil {
		return nil, err
	}
	if err := s.tx.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) BulkCreate(ctx context.Context, transactions []*domain.Transaction) ([]*domain.Transaction, error) {
	if err := s.tx.SaveMany(ctx, transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Transaction, error) {
	return s.tx.FindAll(ctx)
}

func (s *Service) Get(ctx context.Context, id uint) (*domain.Transaction, error) {
	return s.tx.FindByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uint, categoryID uint, amount int64, txType domain.TransactionType, note string, date time.Time) (*domain.Transaction, error) {
	t, err := s.tx.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if categoryID != 0 {
		t.SetCategoryID(categoryID)
	}

	if amount != 0 {
		t.SetAmount(amount)
	}
	if txType != "" {
		t.SetType(txType)
	}
	if note != "" {
		t.SetNote(note)
	}
	if !date.IsZero() {
		t.SetDate(date)
	}
	if err := s.tx.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.tx.Delete(ctx, id)
}

// Transaction Category Service

func (s *Service) CreateCategory(ctx context.Context, name string) (*domain.TransactionCategory, error) {
	c, err := domain.NewTransactionCategory(name)
	if err != nil {
		return nil, err
	}
	if err := s.cat.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]*domain.TransactionCategory, error) {
	return s.cat.FindAll(ctx)
}

func (s *Service) GetCategory(ctx context.Context, id uint) (*domain.TransactionCategory, error) {
	return s.cat.FindByID(ctx, id)
}

func (s *Service) UpdateCategory(ctx context.Context, id uint, name string) (*domain.TransactionCategory, error) {
	c, err := s.cat.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		c.Rename(name)
	}
	if err := s.cat.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	return s.cat.Delete(ctx, id)
}
