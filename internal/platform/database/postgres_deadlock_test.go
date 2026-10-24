package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func TestPostgresWholeOperationRetriesRealDeadlock(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	held := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for _, keys := range [][2]int64{{72173801, 72173802}, {72173802, 72173801}} {
		go func() {
			for attempt := 0; ; attempt++ {
				err := func() error {
					tx, err := s.db.BeginTx(ctx, nil)
					if err != nil {
						return err
					}
					defer tx.Rollback()
					if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(?)`, keys[0]); err != nil {
						return err
					}
					if attempt == 0 {
						held <- struct{}{}
						select {
						case <-release:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(?)`, keys[1]); err != nil {
						return err
					}
					return tx.Commit()
				}()
				if !s.retryTransaction(ctx, err, attempt) {
					results <- err
					return
				}
			}
		}()
	}
	for range 2 {
		select {
		case <-held:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if postgres.Snapshot(s.db.Driver()).Retries < 1 {
		t.Fatal("deadlock did not exercise whole-operation retry")
	}
}
