package domain

import "errors"

var (
	ErrNameRequired       = errors.New("routine name is required")
	ErrNameTooLong        = errors.New("routine name too long")
	ErrDescriptionTooLong = errors.New("routine description too long")
	ErrRoutineNotFound    = errors.New("routine not found")
	ErrDuplicateName      = errors.New("routine name already exists")
)
