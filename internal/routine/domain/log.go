package domain

import "time"

type RoutineLog struct {
	id          uint
	routineID   uint
	completedAt time.Time
}

func LogCompletion(r *Routine) *RoutineLog {
	return &RoutineLog{
		routineID:   r.id,
		completedAt: time.Now(),
	}
}

func HydrateLog(id, routineID uint, completedAt time.Time) *RoutineLog {
	return &RoutineLog{id: id, routineID: routineID, completedAt: completedAt}
}

func (l *RoutineLog) ID() uint {
	return l.id
}
func (l *RoutineLog) RoutineID() uint {
	return l.routineID
}
func (l *RoutineLog) CompletedAt() time.Time {
	return l.completedAt
}
