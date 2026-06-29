package domain

import "errors"

var ErrNegativeAmount = errors.New("amoun cannot be negative")

type Money struct {
	cents int64
}

func NewMoney(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, ErrNegativeAmount
	}
	return Money{cents: cents}, nil
}

func (m Money) Cents() int64      { return m.cents }
func (m Money) Add(o Money) Money { return Money{cents: m.cents + o.cents} }
