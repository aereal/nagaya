package nagaya

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type newConfig struct {
	tp trace.TracerProvider
}

type NewOption interface {
	applyNewOption(cfg *newConfig)
}

type middlewareConfig struct {
	tp                trace.TracerProvider
	reqIDGen          RequestIDGenerator
	decideTenant      DecideTenantFn
	errorHandler      ErrorHandler
	bindConnectionCfg *bindConnectionConfig
}

// MiddlewareOption applies a configuration option value to a middleware.
type MiddlewareOption interface {
	applyMiddlewareOption(cfg *middlewareConfig)
}

type bindConnectionConfig struct {
	changeTenantTimeout time.Duration
}

type BindConnectionOption interface {
	applyBindConnectionOption(cfg *bindConnectionConfig)
}

type optTracerProvider struct{ tp trace.TracerProvider }

func (o *optTracerProvider) applyNewOption(cfg *newConfig) {
	cfg.tp = o.tp
}

func (o *optTracerProvider) applyMiddlewareOption(cfg *middlewareConfig) { cfg.tp = o.tp }

// WithTracerProvider creates an Option tells that use given TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) interface {
	NewOption
	MiddlewareOption
} {
	return &optTracerProvider{tp: tp}
}

type optTimeout struct{ dur time.Duration }

func (o *optTimeout) applyMiddlewareOption(cfg *middlewareConfig) {
	if cfg.bindConnectionCfg == nil {
		cfg.bindConnectionCfg = new(bindConnectionConfig)
	}
	o.applyBindConnectionOption(cfg.bindConnectionCfg)
}

func (o *optTimeout) applyBindConnectionOption(cfg *bindConnectionConfig) {
	cfg.changeTenantTimeout = o.dur
}

// WithTimeout sets the how long wait for a tenant change.
func WithTimeout(dur time.Duration) interface {
	MiddlewareOption
	BindConnectionOption
} {
	return &optTimeout{dur: dur}
}

// GetTenantFromHeader tells the middleware get the tenant from the request header.
//
// Deprecated: use DecideTenantFromHeader
func GetTenantFromHeader(headerName string) MiddlewareOption {
	return DecideTenantFromHeader(headerName)
}

// WithGetTenantFn tells the middleware get the tenant using given function.
//
// Deprecated: use WithDecideTenantFn
func WithGetTenantFn(fn func(r *http.Request) (Tenant, bool)) MiddlewareOption {
	return WithDecideTenantFn(func(r *http.Request) TenantDecisionResult {
		tenant, ok := fn(r)
		if !ok {
			return &TenantDecisionResultError{Err: ErrNoTenantBound}
		}
		return &TenantDecisionResultChangeTenant{Tenant: tenant}
	})
}

type optDecideTenantFn struct {
	fn DecideTenantFn
}

func (o *optDecideTenantFn) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.decideTenant = o.fn
}

// WithDecideTenantFn tells the middleware to use given function to decide the tenant.
func WithDecideTenantFn(fn DecideTenantFn) MiddlewareOption { return &optDecideTenantFn{fn: fn} }

// DecideTenantFromHeader tells the middleware to use given header value to decide the tenant.
func DecideTenantFromHeader(headerName string) MiddlewareOption {
	return &optDecideTenantFn{
		fn: func(r *http.Request) TenantDecisionResult {
			tenant := r.Header.Get(headerName)
			if tenant == "" {
				return &TenantDecisionResultError{Err: ErrNoTenantBound}
			}
			return &TenantDecisionResultChangeTenant{Tenant: Tenant(tenant)}
		},
	}
}

type optRequestIDGenerator struct{ gen RequestIDGenerator }

func (o *optRequestIDGenerator) applyMiddlewareOption(cfg *middlewareConfig) { cfg.reqIDGen = o.gen }

// WithRequestIDGenerator tells the middleware to use given [RequestIDGenerator].
func WithRequestIDGenerator(gen RequestIDGenerator) MiddlewareOption {
	return &optRequestIDGenerator{gen: gen}
}

type optErrorHandler struct{ handler ErrorHandler }

func (o *optErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.errorHandler = o.handler
}

// WithErrorHandler tells the middleware to use given error handler for all errors.
func WithErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optErrorHandler{handler: handler}
}

// WithChangeTenantErrorHandler is deprecated.
//
// Deprecated: use WithErrorHandler
func WithChangeTenantErrorHandler(handler ErrorHandler) MiddlewareOption {
	return WithErrorHandler(handler)
}

// WithGenerateRequestIDErrorHandler is deprecated.
//
// Deprecated: use WithErrorHandler
func WithGenerateRequestIDErrorHandler(handler ErrorHandler) MiddlewareOption {
	return WithErrorHandler(handler)
}

// WithNoTenantBoundErrorHandler is deprecated.
//
// Deprecated: use WithErrorHandler
func WithNoTenantBoundErrorHandler(handler ErrorHandler) MiddlewareOption {
	return WithErrorHandler(handler)
}

// WithObtainConnectionErrorHandler is deprecated.
//
// Deprecated: use WithErrorHandler
func WithObtainConnectionErrorHandler(handler ErrorHandler) MiddlewareOption {
	return WithErrorHandler(handler)
}
