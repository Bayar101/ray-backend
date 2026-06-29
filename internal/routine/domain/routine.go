package domain

import "time"

type Routine struct {
	id          uint
	name        string
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

func NewRoutine(name, description string) (*Routine, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(name) > 100 {
		return nil, ErrNameTooLong
	}
	return &Routine{name: name, description: description}, nil
}

func (r *Routine) Rename(name string) error {
	if name == "" {
		return ErrNameRequired
	}
	r.name = name
	return nil
}

func (r *Routine) Describe(description string) error {
	if len(description) > 1000 {
		return ErrDescriptionTooLong
	}
	r.description = description
	return nil
}

func Hydrate(id uint, name, description string) *Routine {
	return &Routine{id: id, name: name, description: description}
}

func (r *Routine) ID() uint            { return r.id }
func (r *Routine) Name() string        { return r.name }
func (r *Routine) Description() string { return r.description }
