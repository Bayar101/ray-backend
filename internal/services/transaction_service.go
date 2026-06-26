package services

import (
	"context"
	"fmt"

	"github.com/Bayar101/ray-backend/internal/models"
	"gorm.io/gorm"
)

type TransactionService struct {
	db *gorm.DB
}

func NewTransactionService(db *gorm.DB) *TransactionService {
	return &TransactionService{db: db}
}

// #region [Create]
func (s *TransactionService) Create(ctx context.Context, t models.Transaction) (models.Transaction, error) {
	if err := s.db.WithContext(ctx).Create(&t).Error; err != nil {
		return models.Transaction{}, fmt.Errorf("failed to create transaction: %w", err)
	}
	return t, nil
}

func (s *TransactionService) BulkCreate(ctx context.Context, ts []models.Transaction) ([]models.Transaction, error) {
	if err := s.db.WithContext(ctx).Create(&ts).Error; err != nil {
		return nil, fmt.Errorf("failed to create transactions: %w", err)
	}
	return ts, nil
}

func (s *TransactionService) CreateCategory(ctx context.Context, c models.TransactionCategory) (models.TransactionCategory, error) {
	if err := s.db.WithContext(ctx).Create(&c).Error; err != nil {
		return models.TransactionCategory{}, fmt.Errorf("failed to create transaction category: %w", err)
	}
	return c, nil
}

// #region [List]
func (s *TransactionService) List(ctx context.Context) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := s.db.WithContext(ctx).Find(&transactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	return transactions, nil
}

func (s *TransactionService) ListCategories(ctx context.Context) ([]models.TransactionCategory, error) {
	var categories []models.TransactionCategory
	if err := s.db.WithContext(ctx).Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to get transaction categories: %w", err)
	}
	return categories, nil
}

// #region [Get]
func (s *TransactionService) Get(ctx context.Context, id uint) (models.Transaction, error) {
	var transaction models.Transaction
	if err := s.db.WithContext(ctx).First(&transaction, id).Error; err != nil {
		return models.Transaction{}, fmt.Errorf("failed to get transaction: %w", err)
	}
	return transaction, nil
}

func (s *TransactionService) GetCategory(ctx context.Context, id uint) (models.TransactionCategory, error) {
	var category models.TransactionCategory
	if err := s.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return models.TransactionCategory{}, fmt.Errorf("failed to get transaction category: %w", err)
	}
	return category, nil
}

// #region [Update]
func (s *TransactionService) Update(ctx context.Context, id uint, t models.Transaction) (models.Transaction, error) {
	var transaction models.Transaction
	if err := s.db.WithContext(ctx).First(&transaction, id).Error; err != nil {
		return models.Transaction{}, fmt.Errorf("failed to get transaction: %w", err)
	}
	if t.CategoryID != 0 {
		transaction.CategoryID = t.CategoryID
	}
	if t.Amount != 0 {
		transaction.Amount = t.Amount
	}
	if t.Type != "" {
		transaction.Type = t.Type
	}
	if err := s.db.WithContext(ctx).Save(&transaction).Error; err != nil {
		return models.Transaction{}, fmt.Errorf("failed to update transaction: %w", err)
	}
	return transaction, nil
}

func (s *TransactionService) UpdateCategory(ctx context.Context, id uint, c models.TransactionCategory) (models.TransactionCategory, error) {
	var category models.TransactionCategory
	if err := s.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return models.TransactionCategory{}, fmt.Errorf("failed to get transaction category: %w", err)
	}
	if c.Name != "" {
		category.Name = c.Name
	}
	if err := s.db.WithContext(ctx).Save(&category).Error; err != nil {
		return models.TransactionCategory{}, fmt.Errorf("failed to update transaction category: %w", err)
	}
	return category, nil
}

// #region [Delete]
func (s *TransactionService) Delete(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.Transaction{}, id)
	if res.Error != nil {
		return fmt.Errorf("failed to delete transaction: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *TransactionService) DeleteCategory(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.TransactionCategory{}, id)
	if res.Error != nil {
		return fmt.Errorf("failed to delete transaction category: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// #endregion
