package nagaya

import (
	"context"
	"errors"
)

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
