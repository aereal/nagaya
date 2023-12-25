package nagaya

import (
	"context"
	"database/sql"
	"time"
)

type DBish interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	PingContext(context.Context) error
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	Close() error
	Stats() sql.DBStats
	SetConnMaxIdleTime(time.Duration)
	SetConnMaxLifetime(time.Duration)
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
}

type Connish interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	PingContext(context.Context) error
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	Close() error
}
