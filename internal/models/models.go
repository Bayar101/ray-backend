package models

func AllModels() []any {
	return []any{
		&Routine{},
		&RoutineLog{},
		&TransactionCategory{},
		&Transaction{},
	}
}
