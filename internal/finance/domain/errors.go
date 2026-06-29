package domain

import "errors"

var (
	ErrTransactionCategoryRequired     = errors.New("transaction category is required")
	ErrInvalidAmount                   = errors.New("invalid amount")
	ErrInvalidType                     = errors.New("invalid type")
	ErrTransactionNotFound             = errors.New("transaction not found")
	ErrNoteTooLong                     = errors.New("note too long")
	ErrTransactionCategoryNotFound     = errors.New("transaction category not found")
	ErrDuplicateTransactionCategory    = errors.New("transaction category name already exists")
	ErrTransactionCategoryNameRequired = errors.New("transaction category name is required")
)
