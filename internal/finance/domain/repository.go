package domain

import "context"

type TransactionRepository interface {
	Save(ctx context.Context, t *Transaction) error
	SaveMany(ctx context.Context, ts []*Transaction) error
	FindByID(ctx context.Context, id uint) (*Transaction, error)
	FindAll(ctx context.Context) ([]*Transaction, error)
	Delete(ctx context.Context, id uint) error
}

type TransactionCategoryRepository interface {
	Save(ctx context.Context, c *TransactionCategory) error
	FindByID(ctx context.Context, id uint) (*TransactionCategory, error)
	FindAll(ctx context.Context) ([]*TransactionCategory, error)
	Delete(ctx context.Context, id uint) error
}
