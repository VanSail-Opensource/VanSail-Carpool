package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"
)

type connector struct {
	driver.Connector
	metrics *transactionMetrics
}

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &connection{Conn: conn, metrics: c.metrics}, nil
}

type connection struct {
	driver.Conn
	metrics *transactionMetrics
}

func (c *connection) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *connection) Prepare(query string) (driver.Stmt, error) {
	// Domain statements use context entry points; reject argument-less binding
	// guesses during preparation. Authored PostgreSQL statements still work.
	bound, err := Bind(query, nil)
	if err != nil {
		return nil, err
	}
	return c.Conn.Prepare(bound)
}

func (c *connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if opts.Isolation == 0 && !opts.ReadOnly {
		opts.Isolation = driver.IsolationLevel(sql.LevelSerializable)
	} // database/sql.LevelSerializable
	started := time.Now()
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &measuredTransaction{Tx: tx, metrics: c.metrics, started: started}, nil
}

func (c *connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	bound, err := Bind(query, args)
	if err != nil {
		return nil, err
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, bound, args)
}

func (c *connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	bound, err := Bind(query, args)
	if err != nil {
		return nil, err
	}
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, bound, args)
	if err != nil {
		return nil, err
	}
	return compatRows{rows}, nil
}

func (c *connection) Ping(ctx context.Context) error { return c.Conn.(driver.Pinger).Ping(ctx) }
func (c *connection) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}
func (c *connection) IsValid() bool {
	validator, ok := c.Conn.(driver.Validator)
	return !ok || validator.IsValid()
}

type compatRows struct{ driver.Rows }

func (r compatRows) Next(values []driver.Value) error {
	if err := r.Rows.Next(values); err != nil {
		return err
	}
	for i, value := range values {
		if flag, ok := value.(bool); ok {
			if flag {
				values[i] = int64(1)
			} else {
				values[i] = int64(0)
			}
		}
	}
	return nil
}

// Is reports whether a sql.DB driver is the PostgreSQL compatibility connector.
func Is(dbDriver driver.Driver) bool { _, ok := dbDriver.(compatDriver); return ok }

type compatDriver struct {
	driver.Driver
	metrics *transactionMetrics
}

func (c connector) Driver() driver.Driver { return compatDriver{c.Connector.Driver(), c.metrics} }
func (compatDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the PostgreSQL connector")
}
