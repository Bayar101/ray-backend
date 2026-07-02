package domain

type Summary struct {
	totalIncome  int64
	totalExpense int64
	categories   []CategorySummary
}

type CategorySummary struct {
	ID           uint
	Name         string
	TotalIncome  int64
	TotalExpense int64
}

func NewSummary(totalIncome, totalExpense int64, categories []CategorySummary) *Summary {
	return &Summary{
		totalIncome:  totalIncome,
		totalExpense: totalExpense,
		categories:   categories,
	}
}

func (s *Summary) TotalIncome() int64            { return s.totalIncome }
func (s *Summary) TotalExpense() int64           { return s.totalExpense }
func (s *Summary) Categories() []CategorySummary { return s.categories }
