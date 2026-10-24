package database

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"

type randomChoice struct {
	bound, value int64
	err          error
}
type transactionRandom struct {
	source   activity.RandomSource
	choices  []randomChoice
	position int
}

func newTransactionRandom(source activity.RandomSource) *transactionRandom {
	if source == nil {
		return nil
	}
	return &transactionRandom{source: source}
}

// Int63n preserves a logical operation's random outcomes across SQL rollback.
// A changed configuration/range refuses reuse rather than silently resampling.
func (r *transactionRandom) Int63n(bound int64) (int64, error) {
	if r.position < len(r.choices) {
		choice := r.choices[r.position]
		r.position++
		if choice.bound != bound {
			return 0, ErrConflict
		}
		return choice.value, choice.err
	}
	value, err := r.source.Int63n(bound)
	r.choices = append(r.choices, randomChoice{bound: bound, value: value, err: err})
	r.position++
	return value, err
}
