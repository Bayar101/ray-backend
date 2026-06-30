package domain

type TransactionCategory struct {
	id   uint
	name string
}

func NewTransactionCategory(name string) (*TransactionCategory, error) {
	if name == "" {
		return nil, ErrTransactionCategoryNameRequired
	}
	return &TransactionCategory{name: name}, nil
}

func (c *TransactionCategory) Rename(name string) error {
	if name == "" {
		return ErrTransactionCategoryNameRequired
	}
	c.name = name
	return nil
}

func HydrateTransactionCategory(id uint, name string) *TransactionCategory {
	return &TransactionCategory{
		id:   id,
		name: name,
	}
}

func (c *TransactionCategory) ID() uint     { return c.id }
func (c *TransactionCategory) Name() string { return c.name }
