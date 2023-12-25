package nagaya

import (
	"context"
	"database/sql"
	"sync"
)

// Tenant is an identifier of resource subset stored in the shared database.
type Tenant string

type tenantCtxKey struct{}

// WithTenant returns new context that contains given tenant.
func WithTenant(ctx context.Context, tenant Tenant) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenant)
}

// TenantFromContext extracts a tenant in the context.
//
// If no tenant is bound for the context, the second return value is a false.
func TenantFromContext(ctx context.Context) (Tenant, bool) {
	tenant, ok := ctx.Value(tenantCtxKey{}).(Tenant)
	return tenant, ok
}

// GetConnFn is a function type returns new database connection from the DB.
type GetConnFn[DB DBish, Conn Connish] func(ctx context.Context, db DB) (Conn, error)

func New[DB DBish, Conn Connish](db DB, getConn GetConnFn[DB, Conn]) *Nagaya[DB, Conn] {
	n := &Nagaya[DB, Conn]{db: db, conns: make(map[string]Conn), getConn: getConn}
	return n
}

func NewStd(db *sql.DB) *Nagaya[*sql.DB, *sql.Conn] {
	return New[*sql.DB, *sql.Conn](db, getConnStd)
}

func getConnStd(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	return db.Conn(ctx)
}

type Nagaya[DB DBish, Conn Connish] struct {
	mux     sync.RWMutex
	db      DB
	conns   map[string]Conn
	getConn GetConnFn[DB, Conn]
}

// ObtainConnection returns a database connection connected to the current tenant.
func (n *Nagaya[DB, Conn]) ObtainConnection(ctx context.Context) (conn Conn, err error) {
	reqID, ok := reqIDFromContext(ctx)
	if !ok {
		err = ErrNoConnectionBound
		return
	}
	n.mux.RLock()
	defer n.mux.RUnlock()
	conn, ok = n.conns[reqID]
	if !ok {
		err = ErrNoConnectionBound
		return
	}
	return
}
