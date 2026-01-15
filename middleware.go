package nagaya

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rs/xid"
	"go.opentelemetry.io/otel/trace"
)

var defaultChangeTenantTimeout = time.Second * 5

var failedToDetermineTenantResult = &TenantDecisionResultError{Err: ErrNoTenantBound}

func failsToDetermineTenant(_ *http.Request) TenantDecisionResult {
	return failedToDetermineTenantResult
}

// Middleware returns a middleware function that determines target tenant and obtain the database connection against the tenant.
//
// The consumer must get the obtained connection via Nagaya.ObtainConnection method and use it to access the database.
func Middleware[DB DBish, Conn Connish](n *Nagaya[DB, Conn], opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := &middlewareConfig{bindConnectionCfg: new(bindConnectionConfig)}
	for _, o := range opts {
		o.applyMiddlewareOption(cfg)
	}
	if cfg.decideTenant == nil {
		cfg.decideTenant = failsToDetermineTenant
	}
	if cfg.reqIDGen == nil {
		cfg.reqIDGen = defaultIDGenerator
	}
	if cfg.errorHandler == nil {
		cfg.errorHandler = jsonErrorHandler
	}
	tracer := getTracer(cfg.tp)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), "Nagaya.Middleware", trace.WithSpanKind(trace.SpanKindServer))
			handler := func(ctx context.Context) error {
				finishSpan(span, nil)
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			}
			d := newDoer(n, handler, &optTenantDecisionResult{cfg.decideTenant(r)}, WithTimeout(cfg.bindConnectionCfg.changeTenantTimeout))
			if timeout := cfg.bindConnectionCfg.changeTenantTimeout; timeout != 0 {
				d.bindConnectionOption = append(d.bindConnectionOption, WithTimeout(timeout))
			}
			if err := d.do(ctx); err != nil {
				cfg.errorHandler(w, r, err)
				finishSpan(span, err)
			}
		})
	}
}

type RequestIDGenerator interface {
	GenerateID() (string, error)
}

type RequestIDGeneratorFunc func() (string, error)

var _ RequestIDGenerator = (RequestIDGeneratorFunc)(nil)

func (f RequestIDGeneratorFunc) GenerateID() (string, error) {
	return f()
}

var defaultIDGenerator = RequestIDGeneratorFunc(func() (string, error) { return xid.New().String(), nil })

// TenantDecision indicates whether a tenant is changed, cannot be changed due to some error, or unchanged.
type TenantDecision int

const (
	// TenantDecisionNoChange will use default tenant.
	TenantDecisionNoChange TenantDecision = iota
	// TenantDecisionError has an intention to change the tenant but failed to determine it.
	TenantDecisionError
	// TenantDecisionChangeTenant will change a tenant.
	TenantDecisionChangeTenant
)

// TenantDecisionResult conveys a TenantDecision and a tenant.
type TenantDecisionResult interface {
	isTenantDecisionResult()
	Decision() TenantDecision
	DecideTenant() (Tenant, error)
}

type TenantDecisionResultNoChange struct{}

var _ TenantDecisionResult = (*TenantDecisionResultNoChange)(nil)

func (TenantDecisionResultNoChange) isTenantDecisionResult() {}

func (TenantDecisionResultNoChange) Decision() TenantDecision { return TenantDecisionNoChange }

func (TenantDecisionResultNoChange) DecideTenant() (Tenant, error) { return "", ErrNoTenantChange }

type TenantDecisionResultError struct{ Err error }

var _ TenantDecisionResult = (*TenantDecisionResultError)(nil)

func (TenantDecisionResultError) isTenantDecisionResult() {}

func (TenantDecisionResultError) Decision() TenantDecision { return TenantDecisionError }

func (r *TenantDecisionResultError) DecideTenant() (Tenant, error) { return "", r.Err }

type TenantDecisionResultChangeTenant struct{ Tenant Tenant }

var _ TenantDecisionResult = (*TenantDecisionResultChangeTenant)(nil)

func (TenantDecisionResultChangeTenant) isTenantDecisionResult() {}

func (TenantDecisionResultChangeTenant) Decision() TenantDecision { return TenantDecisionChangeTenant }

func (r *TenantDecisionResultChangeTenant) DecideTenant() (Tenant, error) { return r.Tenant, nil }

type DecideRequestTenantFunc func(*http.Request) TenantDecisionResult

// DecideTenantFn is deprecated.
//
// Deprecated: use [DecideRequestTenantFunc].
type DecideTenantFn = DecideRequestTenantFunc

type reqIDCtxKey struct{}

func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDCtxKey{}, id)
}

func reqIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(reqIDCtxKey{}).(string)
	return id, ok
}

// ErrorHandler is a function that called if the error occurred.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

func jsonErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("content-security-policy", "default-src 'none'")
	w.Header().Set("x-content-type-options", "nosniff")
	w.Header().Set("x-frame-options", "DENY")
	status := http.StatusInternalServerError
	if errors.Is(err, ErrNoConnectionBound) {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck,errchkjson
}
