package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Raw repository helpers remain available to validated workers and fixtures.
// Administrator adapters hold current authority until mutation and audit commit.
func xnodeAdminActorTx(ctx context.Context, tx *sql.Tx, actor string) error {
	if err := catalogAdminActorTx(ctx, tx, actor); errors.Is(err, ErrConflict) {
		return xnode.ErrConflict
	} else {
		return err
	}
}

type xnodeSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type xnodeSQLReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
