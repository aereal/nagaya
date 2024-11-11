package nagaya

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/trace"
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

func New[DB DBish, Conn Connish](db DB, getConn GetConnFn[DB, Conn], opts ...NewOption) *Nagaya[DB, Conn] {
	cfg := new(newConfig)
	for _, o := range opts {
		o.applyNewOption(cfg)
	}
	tracer := getTracer(cfg.tp)

	n := &Nagaya[DB, Conn]{db: db, conns: make(map[string]Conn), getConn: getConn, tracer: tracer}
	return n
}

func NewStd(db *sql.DB, opts ...NewOption) *Nagaya[*sql.DB, *sql.Conn] {
	return New[*sql.DB, *sql.Conn](db, getConnStd, opts...)
}

func getConnStd(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	return db.Conn(ctx)
}

type Nagaya[DB DBish, Conn Connish] struct {
	tracer  trace.Tracer
	db      DB
	conns   map[string]Conn
	getConn GetConnFn[DB, Conn]
	mux     sync.RWMutex
}

// ObtainConnection returns a database connection bound to the current tenant.
//
// [BindConnection] must be called before this method called.
// Almost users just use [Middleware] that calls [BindConnection].
func (n *Nagaya[DB, Conn]) ObtainConnection(ctx context.Context) (conn Conn, err error) {
	ctx, span := n.tracer.Start(ctx, "Nagaya.ObtainConnection")
	defer finishSpan(span, err)

	reqID, ok := reqIDFromContext(ctx)
	if !ok {
		err = ErrNoConnectionBound
		return
	}
	span.SetAttributes(attrRequestID(reqID))
	n.mux.RLock()
	defer n.mux.RUnlock()
	conn, ok = n.conns[reqID]
	if !ok {
		err = ErrNoConnectionBound
		return
	}
	return
}

// BindConnection returns a new connection from the DB that bound for given tenant.
//
// Usually the users should use [Middleware].
func (n *Nagaya[DB, Conn]) BindConnection(ctx context.Context, tenant Tenant, opts ...BindConnectionOption) (c Conn, err error) {
	ctx, span := n.tracer.Start(ctx, "Nagaya.BindConnection", trace.WithAttributes(attrTenant(tenant)))
	defer finishSpan(span, err)

	var cfg bindConnectionConfig
	for _, o := range opts {
		o.applyBindConnectionOption(&cfg)
	}
	if cfg.changeTenantTimeout == 0 {
		cfg.changeTenantTimeout = defaultChangeTenantTimeout
	}

	requestID, ok := reqIDFromContext(ctx)
	if !ok {
		return c, ErrNoConnectionBound
	}
	span.SetAttributes(attrRequestID(requestID))
	conn, err := n.getConn(ctx, n.db)
	if err != nil {
		return c, &ObtainConnectionError{err: err}
	}
	exCtx, cancel := context.WithTimeout(ctx, cfg.changeTenantTimeout)
	defer cancel()
	if _, err := conn.ExecContext(exCtx, fmt.Sprintf("use %s", tenant)); err != nil {
		return c, &ChangeTenantError{err: err, tenant: tenant}
	}
	n.mux.Lock()
	n.conns[requestID] = conn
	n.mux.Unlock()
	return conn, nil
}

// ReleaseConnection marks the current request's connection is ready to discard.
//
// This method does not call [sql.Conn.Close], it is caller's responsibility.
func (n *Nagaya[DB, Conn]) ReleaseConnection(requestID string) {
	n.mux.Lock()
	defer n.mux.Unlock()
	delete(n.conns, requestID)
}
