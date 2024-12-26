package nagayagqlgen

import (
	"context"
	"errors"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/nagaya"
	"github.com/rs/xid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkg                        = "github.com/aereal/nagaya/github.com/99designs/gqlgen/nagayagqlgen"
	defaultChangeTenantTimeout = time.Second * 5
)

// ErrorHandler is a function that called if the error occurred.
type ErrorHandler func(ctx context.Context, next graphql.ResponseHandler, err error) *graphql.Response

// TenantDecider is a type that decides
type TenantDecider interface {
	DecideTenant(*graphql.OperationContext) nagaya.TenantDecisionResult
}

type TenantDeciderFunc func(*graphql.OperationContext) nagaya.TenantDecisionResult

var _ TenantDecider = (TenantDeciderFunc)(nil)

func (f TenantDeciderFunc) DecideTenant(opCtx *graphql.OperationContext) nagaya.TenantDecisionResult {
	return f(opCtx)
}

type RequestIDGenerator interface {
	GenerateID(ctx context.Context, opCtx *graphql.OperationContext) (string, error)
}

type RequestIDGeneratorFunc func(ctx context.Context, opCtx *graphql.OperationContext) (string, error)

func (f RequestIDGeneratorFunc) GenerateID(ctx context.Context, opCtx *graphql.OperationContext) (string, error) {
	return f(ctx, opCtx)
}

func failsToDetermineTenant(_ *graphql.OperationContext) nagaya.TenantDecisionResult {
	return &nagaya.TenantDecisionResultError{Err: nagaya.ErrNoTenantBound}
}

func defaultErrorHandler(ctx context.Context, _ graphql.ResponseHandler, err error) *graphql.Response {
	return graphql.ErrorResponse(ctx, "%s", err)
}

func defaultIDGenerator(_ context.Context, _ *graphql.OperationContext) (string, error) {
	return xid.New().String(), nil
}

func NewExtension[DB nagaya.DBish, Conn nagaya.Connish](ngy *nagaya.Nagaya[DB, Conn], opts ...Option) *Extension[DB, Conn] {
	cfg := new(extensionConfig)
	for _, o := range opts {
		o(cfg)
	}
	if cfg.changeTenantTimeout == 0 {
		cfg.changeTenantTimeout = defaultChangeTenantTimeout
	}
	if cfg.tenantDecider == nil {
		cfg.tenantDecider = TenantDeciderFunc(failsToDetermineTenant)
	}
	if cfg.reqIDGen == nil {
		cfg.reqIDGen = RequestIDGeneratorFunc(defaultIDGenerator)
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}
	if cfg.errorHandler == nil {
		cfg.errorHandler = defaultErrorHandler
	}

	ext := &Extension[DB, Conn]{
		ngy:                 ngy,
		tracer:              cfg.tp.Tracer(pkg),
		changeTenantTimeout: cfg.changeTenantTimeout,
		tenantDecider:       cfg.tenantDecider,
		reqIDGen:            cfg.reqIDGen,
		errorHandler:        cfg.errorHandler,
	}
	return ext
}

type Extension[DB nagaya.DBish, Conn nagaya.Connish] struct {
	ngy                 *nagaya.Nagaya[DB, Conn]
	tracer              trace.Tracer
	changeTenantTimeout time.Duration
	tenantDecider       TenantDecider
	reqIDGen            RequestIDGenerator
	errorHandler        ErrorHandler
}

var (
	_ graphql.HandlerExtension    = (*Extension[nagaya.DBish, nagaya.Connish])(nil)
	_ graphql.ResponseInterceptor = (*Extension[nagaya.DBish, nagaya.Connish])(nil)
)

func (e *Extension[DB, Conn]) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	ctx, span := e.tracer.Start(ctx, "IncerceptResponse", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	opCtx := graphql.GetOperationContext(ctx)
	tenant, err := e.tenantDecider.DecideTenant(opCtx).DecideTenant()
	if errors.Is(err, nagaya.ErrNoTenantChange) {
		return next(ctx)
	}
	if err != nil {
		return e.errorHandler(ctx, next, err)
	}
	span.SetAttributes(nagaya.KeyTenant.String(string(tenant)))

	reqID, err := e.reqIDGen.GenerateID(ctx, opCtx)
	if err != nil {
		return e.errorHandler(ctx, next, err)
	}
	span.SetAttributes(nagaya.KeyRequestID.String(reqID))

	ctx = nagaya.ContextWithRequestID(nagaya.WithTenant(ctx, tenant), reqID)
	changeTenantCtx, cancel := context.WithTimeout(ctx, e.changeTenantTimeout)
	defer cancel()
	conn, err := e.ngy.BindConnection(changeTenantCtx, tenant)
	if err != nil {
		return e.errorHandler(ctx, next, err)
	}
	defer func() {
		_ = conn.Close()
		e.ngy.ReleaseConnection(reqID)
	}()
	return next(ctx)
}

func (Extension[_, _]) ExtensionName() string { return pkg + ".Extension" }

func (Extension[_, _]) Validate(graphql.ExecutableSchema) error { return nil }
