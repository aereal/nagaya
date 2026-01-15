package nagaya

import (
	"context"
	"errors"
)

// Do runs a handler with the database connection that bound for the determined tenant.
func Do[DB DBish, Conn Connish](ctx context.Context, n *Nagaya[DB, Conn], handler func(context.Context) error, opts ...DoOption) error {
	return newDoer(n, handler, opts...).do(ctx)
}

// Yield returns a value from a yielder function.
func Yield[V any, DB DBish, Conn Connish](ctx context.Context, n *Nagaya[DB, Conn], yielder func(context.Context) (V, error), opts ...DoOption) (V, error) {
	var ret V
	handler := func(ctx context.Context) error {
		var err error
		ret, err = yielder(ctx)
		return err
	}
	if err := newDoer(n, handler, opts...).do(ctx); err != nil {
		return ret, err
	}
	return ret, nil
}

func newDoer[DB DBish, Conn Connish](n *Nagaya[DB, Conn], handler func(context.Context) error, opts ...DoOption) *doer[DB, Conn] {
	var cfg doConfig
	for _, o := range opts {
		o.applyDoOption(&cfg)
	}
	if cfg.reqIDGen == nil {
		cfg.reqIDGen = defaultIDGenerator
	}
	if cfg.tenantDecisionRet == nil {
		cfg.tenantDecisionRet = failedToDetermineTenantResult
	}
	return &doer[DB, Conn]{
		n:                    n,
		decisionResult:       cfg.tenantDecisionRet,
		handler:              handler,
		idGenerator:          cfg.reqIDGen,
		bindConnectionOption: cfg.bindConnectionOpts,
	}
}

type doer[DB DBish, Conn Connish] struct {
	n                    *Nagaya[DB, Conn]
	decisionResult       TenantDecisionResult
	idGenerator          RequestIDGenerator
	handler              func(context.Context) error
	bindConnectionOption []BindConnectionOption
}

func (d *doer[DB, Conn]) do(ctx context.Context) error {
	tenant, err := d.decisionResult.DecideTenant()
	if errors.Is(err, ErrNoTenantChange) {
		return d.handler(ctx)
	}
	if err != nil {
		return err
	}
	id, err := d.idGenerator.GenerateID()
	if err != nil {
		return &GenerateRequestIDError{err: err}
	}
	handlerCtx := ContextWithRequestID(WithTenant(ctx, tenant), id)
	conn, err := d.n.BindConnection(handlerCtx, tenant, d.bindConnectionOption...)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	defer d.n.ReleaseConnection(id)
	return d.handler(handlerCtx)
}
