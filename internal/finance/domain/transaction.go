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
	id                    uint
	transactionCategoryID uint
	amount                Money
	txType                TransactionType
	note                  string
	date                  time.Time
}

func NewTransaction(transactionCategoryID uint, amount Money, txType TransactionType, note string, date time.Time) (*Transaction, error) {
	if transactionCategoryID == 0 {
		return nil, ErrTransactionCategoryRequired
	}
	if amount.Cents() <= 0 {
		return nil, ErrInvalidAmount
	}
	if !txType.Valid() {
		return nil, ErrInvalidType
	}
	if len(note) > 1000 {
		return nil, ErrNoteTooLong
	}
	return &Transaction{
		transactionCategoryID: transactionCategoryID,
		amount:                amount,
		txType:                txType,
		note:                  note,
		date:                  date,
	}, nil
}

func HydrateTransaction(id, transactionCategoryID uint, amount Money, txType TransactionType, note string, date time.Time) *Transaction {
	return &Transaction{
		id:                    id,
		transactionCategoryID: transactionCategoryID,
		amount:                amount,
		txType:                txType,
		note:                  note,
		date:                  date,
	}
}

func (t *Transaction) ID() uint                       { return t.id }
func (t *Transaction) TransactionCategoryID() uint    { return t.transactionCategoryID }
func (t *Transaction) Amount() Money                  { return t.amount }
func (t *Transaction) Type() TransactionType          { return t.txType }
func (t *Transaction) Note() string                   { return t.note }
func (t *Transaction) Date() time.Time                { return t.date }
func (t *Transaction) SetCategoryID(categoryID uint)  { t.transactionCategoryID = categoryID }
func (t *Transaction) SetAmount(amount Money)         { t.amount = amount }
func (t *Transaction) SetType(txType TransactionType) { t.txType = txType }
func (t *Transaction) SetNote(note string)            { t.note = note }
func (t *Transaction) SetDate(date time.Time)         { t.date = date }
