package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresRuntimeOwnershipAndNotifications(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("hosted PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	first, err := AcquireOwner(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if second, e := AcquireOwner(ctx, url); e == nil {
		second.Close()
		first.Close()
		t.Fatal("second master acquired ownership")
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err := AcquireOwner(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	listenCtx, stop := context.WithCancel(ctx)
	go func() { defer close(done); RunNotifications(listenCtx, url, wake, nil) }()
	defer func() { stop(); <-done }()
	select {
	case <-wake:
	case <-ctx.Done():
		t.Fatal("listener did not start")
	}
	conn, err := runtimeConnection(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `NOTIFY vansail_outbox`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wake:
	case <-ctx.Done():
		t.Fatal("notification was not delivered")
	}
	if _, err = conn.Exec(ctx, `SELECT pg_terminate_backend($1)`, owner.conn.PgConn().PID()); err != nil {
		t.Fatal(err)
	}
	if err = owner.Monitor(ctx); err == nil {
		t.Fatal("lost ownership was accepted")
	}
}
