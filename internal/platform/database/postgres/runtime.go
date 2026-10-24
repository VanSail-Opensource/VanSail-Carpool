package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// Owner holds a dedicated session lock for the initial single-master deployment.
// It is not a distributed fencing protocol: deploy one application process.
type Owner struct{ conn *pgx.Conn }

func runtimeConnection(ctx context.Context, url string) (*pgx.Conn, error) {
	parsed, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL runtime configuration")
	}
	parsed.ConnectTimeout = 10 * time.Second
	parsed.RuntimeParams["application_name"] = "vansail-runtime"
	parsed.RuntimeParams["statement_timeout"] = "5000"
	return pgx.ConnectConfig(ctx, parsed)
}

// AcquireOwner refuses an additional scheduler/master before startup mutations.
func AcquireOwner(ctx context.Context, url string) (*Owner, error) {
	conn, err := runtimeConnection(ctx, url)
	if err != nil {
		return nil, errors.New("PostgreSQL ownership connection failed")
	}
	var acquired bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(72173680)`).Scan(&acquired); err != nil || !acquired {
		conn.Close(context.Background())
		return nil, errors.New("PostgreSQL scheduler ownership unavailable; only one application may run")
	}
	return &Owner{conn: conn}, nil
}

// Monitor stops the runtime when its ownership session becomes uncertain.
func (o *Owner) Monitor(ctx context.Context) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			call, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := o.conn.Ping(call)
			cancel()
			if err != nil && ctx.Err() == nil {
				return errors.New("PostgreSQL scheduler ownership connection lost")
			}
		}
	}
}

func (o *Owner) Close() error {
	if o == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return o.conn.Close(ctx)
}

// RunNotifications uses its own reconnecting session. Hints are coalesced and
// carry no business data; durable polling remains required after missed hints.
func RunNotifications(ctx context.Context, url string, wake chan<- struct{}, logger *slog.Logger) {
	delay := time.Second
	for ctx.Err() == nil {
		conn, err := runtimeConnection(ctx, url)
		if err == nil {
			_, err = conn.Exec(ctx, `LISTEN vansail_outbox`)
		}
		if err == nil {
			delay = time.Second
			select {
			case wake <- struct{}{}:
			default:
			}
			for ctx.Err() == nil {
				if _, err = conn.WaitForNotification(ctx); err != nil {
					break
				}
				select {
				case wake <- struct{}{}:
				default:
				}
			}
		}
		if conn != nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			conn.Close(closeCtx)
			cancel()
		}
		if ctx.Err() != nil {
			return
		}
		if logger != nil {
			logger.Warn("PostgreSQL outbox notification session reconnecting", "retry_seconds", int(delay.Seconds()))
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		delay = min(30*time.Second, delay*2)
	}
}
