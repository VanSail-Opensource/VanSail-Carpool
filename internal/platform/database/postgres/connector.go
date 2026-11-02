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
	return c.Conn.Prepare(query)
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
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *connection) Ping(ctx context.Context) error { return c.Conn.(driver.Pinger).Ping(ctx) }

func (c *connection) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}
func (c *connection) IsValid() bool {
	validator, ok := c.Conn.(driver.Validator)
	return !ok || validator.IsValid()
}

type nativeDriver struct {
	driver.Driver
	metrics *transactionMetrics
}

func (c connector) Driver() driver.Driver { return nativeDriver{c.Connector.Driver(), c.metrics} }
func (nativeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the PostgreSQL connector")
}
