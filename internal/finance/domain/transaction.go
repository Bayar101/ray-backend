package domain

import "time"

type TransactionType string

const (
	Income  TransactionType = "income"
	Expense TransactionType = "expense"
)

func (t TransactionType) Valid() bool {
	return t == Income || t == Expense
}

type Transaction struct {
	id         uint
	categoryID uint
	amount     int64
	txType     TransactionType
	note       string
	date       time.Time
}

func NewTransaction(categoryID uint, amount int64, txType TransactionType, note string, date time.Time) (*Transaction, error) {
	if categoryID == 0 {
		return nil, ErrTransactionCategoryRequired
	}
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if !txType.Valid() {
		return nil, ErrInvalidType
	}
	if len(note) > 1000 {
		return nil, ErrNoteTooLong
	}
	if date.IsZero() {
		return nil, ErrInvalidDate
	}
	return &Transaction{
		categoryID: categoryID,
		amount:     amount,
		txType:     txType,
		note:       note,
		date:       date,
	}, nil
}

func (t *Transaction) SetCategoryID(categoryID uint) error {
	if categoryID == 0 {
		return ErrTransactionCategoryRequired
	}
	t.categoryID = categoryID
	return nil
}

func (t *Transaction) SetAmount(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	t.amount = amount
	return nil
}
func (t *Transaction) SetType(txType TransactionType) error {
	if !txType.Valid() {
		return ErrInvalidType
	}
	t.txType = txType
	return nil
}
func (t *Transaction) SetNote(note string) error {
	if len(note) > 1000 {
		return ErrNoteTooLong
	}
	t.note = note
	return nil
}

func (t *Transaction) SetDate(date time.Time) error {
	if date.IsZero() {
		return ErrInvalidDate
	}
	t.date = date
	return nil
}

func HydrateTransaction(id, categoryID uint, amount int64, txType TransactionType, note string, date time.Time) *Transaction {
	return &Transaction{
		id:         id,
		categoryID: categoryID,
		amount:     amount,
		txType:     txType,
		note:       note,
		date:       date,
	}
}

func (t *Transaction) ID() uint              { return t.id }
func (t *Transaction) CategoryID() uint      { return t.categoryID }
func (t *Transaction) Amount() int64         { return t.amount }
func (t *Transaction) Type() TransactionType { return t.txType }
func (t *Transaction) Note() string          { return t.note }
func (t *Transaction) Date() time.Time       { return t.date }
