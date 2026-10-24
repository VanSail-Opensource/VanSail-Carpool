package database

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// retryTransaction repeats only fully rolled-back PostgreSQL operations. Store
// methods own no external calls; providers execute after durable queue commit.
func (s *Store) retryTransaction(ctx context.Context, err error, attempt int) bool {
	if err == nil || attempt >= 15 || !IsPostgres(s.db) || ctx.Err() != nil {
		return false
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || (pg.Code != "40001" && pg.Code != "40P01") {
		return false
	}
	delay := time.Millisecond * time.Duration(2<<min(attempt, 6))
	timer := time.NewTimer(delay/2 + time.Duration(rand.Int64N(int64(delay))))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
