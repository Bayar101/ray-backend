package infra

import (
	"context"
	"errors"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type transactionRecord struct {
	ID                    uint           `gorm:"primaryKey"`
	TransactionCategoryID uint           `gorm:"not null;index"`
	Amount                int64          `gorm:"not null"`
	Type                  string         `gorm:"not null;index;type:varchar(20)"`
	Note                  string         `gorm:"type:text"`
	Date                  time.Time      `gorm:"not null;index"`
	CreatedAt             time.Time      `gorm:"autoCreateTime"`
	UpdatedAt             time.Time      `gorm:"autoUpdateTime"`
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

func (transactionRecord) TableName() string { return "transactions" }

type transactionCategoryRecord struct {
	ID        uint           `gorm:"primaryKey"`
	Name      string         `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (transactionCategoryRecord) TableName() string { return "transaction_categories" }

func Models() []any { return []any{&transactionRecord{}, &transactionCategoryRecord{}} }

// mappers

func txToDomain(rec transactionRecord) *domain.Transaction {
	amount, _ := domain.NewMoney(rec.Amount)
	return domain.HydrateTransaction(rec.ID, rec.TransactionCategoryID, amount, domain.TransactionType(rec.Type), rec.Note, rec.Date)
}

func txToRecord(t *domain.Transaction) transactionRecord {
	return transactionRecord{
		ID:                    t.ID(),
		TransactionCategoryID: t.TransactionCategoryID(),
		Amount:                t.Amount().Cents(),
		Type:                  string(t.Type()),
		Note:                  t.Note(),
		Date:                  t.Date(),
	}
}

func catToDomain(rec transactionCategoryRecord) *domain.TransactionCategory {
	return domain.HydrateTransactionCategory(rec.ID, rec.Name)
}

func catToRecord(c *domain.TransactionCategory) transactionCategoryRecord {
	return transactionCategoryRecord{ID: c.ID(), Name: c.Name()}
}

// Transaction Repository

type TransactionGormRepository struct{ db *gorm.DB }

func NewTransactionGormRepository(db *gorm.DB) *TransactionGormRepository {
	return &TransactionGormRepository{db: db}
}

func (r *TransactionGormRepository) Save(ctx context.Context, t *domain.Transaction) error {
	rec := txToRecord(t)
	if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
		return err
	}
	*t = *txToDomain(rec)
	return nil
}

func (r *TransactionGormRepository) SaveMany(ctx context.Context, ts []*domain.Transaction) error {
	recs := make([]transactionRecord, len(ts))
	for i, t := range ts {
		recs[i] = txToRecord(t)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&recs).Error; err != nil {
			return err
		}
		for i := range ts {
			*ts[i] = *txToDomain(recs[i])
		}
		return nil
	})
}

func (r *TransactionGormRepository) FindByID(ctx context.Context, id uint) (*domain.Transaction, error) {
	var rec transactionRecord
	if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, err
	}
	return txToDomain(rec), nil
}

func (r *TransactionGormRepository) FindAll(ctx context.Context) ([]*domain.Transaction, error) {
	var recs []transactionRecord
	if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.Transaction, len(recs))
	for i, rec := range recs {
		out[i] = txToDomain(rec)
	}
	return out, nil
}

func (r TransactionGormRepository) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&transactionRecord{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTransactionNotFound
	}
	return nil
}

// TransactionCategory Repository

type TransactionCategoryGormRepository struct{ db *gorm.DB }

func NewTransactionCategoryGormRepository(db *gorm.DB) *TransactionCategoryGormRepository {
	return &TransactionCategoryGormRepository{db: db}
}

func (r *TransactionCategoryGormRepository) Save(ctx context.Context, c *domain.TransactionCategory) error {
	rec := catToRecord(c)
	if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateTransactionCategory
		}
		return err
	}
	*c = *catToDomain(rec)
	return nil
}

func (r *TransactionCategoryGormRepository) FindByID(ctx context.Context, id uint) (*domain.TransactionCategory, error) {
	var rec transactionCategoryRecord
	if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTransactionCategoryNotFound
		}
		return nil, err
	}
	return catToDomain(rec), nil
}

func (r *TransactionCategoryGormRepository) FindAll(ctx context.Context) ([]*domain.TransactionCategory, error) {
	var recs []transactionCategoryRecord
	if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.TransactionCategory, len(recs))
	for i, rec := range recs {
		out[i] = catToDomain(rec)
	}
	return out, nil
}

func (r *TransactionCategoryGormRepository) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&transactionCategoryRecord{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTransactionCategoryNotFound
	}
	return nil
}
