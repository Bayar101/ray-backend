package models

import "time"

type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

type TransactionCategory struct {
	Base
	Name string `gorm:"not null;uniqueIndex" json:"name"`
}

type Transaction struct {
	Base
	CategoryID uint                `gorm:"not null;index" json:"category_id"`
	Category   TransactionCategory `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Amount     int64               `gorm:"not null" json:"amount"`
	Type       TransactionType     `gorm:"not null;index;type:varchar(20)" json:"type"`
	Note       string              `gorm:"type:text;" json:"note"`
	Date       time.Time           `gorm:"not null;index" json:"date"`
}
