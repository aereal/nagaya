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

type optTracerProvider struct{ tp trace.TracerProvider }

func (o *optTracerProvider) applyNewOption(cfg *newConfig) {
	cfg.tp = o.tp
}

func (o *optTracerProvider) applyMiddlewareOption(cfg *middlewareConfig) { cfg.tp = o.tp }

func WithTracerProvider(tp trace.TracerProvider) interface {
	NewOption
	MiddlewareOption
} {
	return &optTracerProvider{tp: tp}
}

type middlewareConfig struct {
	tp                           trace.TracerProvider
	reqIDGen                     RequestIDGenerator
	decideTenant                 DecideTenantFn
	handleNoTenantBoundError     ErrorHandler
	handleObtainConnectionError  ErrorHandler
	handleChangeTenantError      ErrorHandler
	handleGenerateRequestIDError ErrorHandler
	changeTenantTimeout          time.Duration
}

// MiddlewareOption applies a configuration option value to a middleware.
type MiddlewareOption interface {
	applyMiddlewareOption(cfg *middlewareConfig)
}

type optTimeout struct{ dur time.Duration }

func (o *optTimeout) applyMiddlewareOption(cfg *middlewareConfig) { cfg.changeTenantTimeout = o.dur }

// WithTimeout configures the time a middleware waits the change of tenant.
func WithTimeout(dur time.Duration) MiddlewareOption {
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

func WithRequestIDGenerator(gen RequestIDGenerator) MiddlewareOption {
	return &optRequestIDGenerator{gen: gen}
}

type optErrorHandler struct{ handler ErrorHandler }

func (o *optErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.handleChangeTenantError = o.handler
	cfg.handleGenerateRequestIDError = o.handler
	cfg.handleNoTenantBoundError = o.handler
	cfg.handleObtainConnectionError = o.handler
}

func WithErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optErrorHandler{handler: handler}
}

type optChangeTenantErrorHandler struct{ handler ErrorHandler }

func (o *optChangeTenantErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.handleChangeTenantError = o.handler
}

func WithChangeTenantErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optChangeTenantErrorHandler{handler: handler}
}

type optGenerateRequestIDErrorHandler struct{ handler ErrorHandler }

func (o *optGenerateRequestIDErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.handleGenerateRequestIDError = o.handler
}

func WithGenerateRequestIDErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optGenerateRequestIDErrorHandler{handler: handler}
}

type optNoTenantBoundErrorHandler struct{ handler ErrorHandler }

func (o *optNoTenantBoundErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.handleNoTenantBoundError = o.handler
}

func WithNoTenantBoundErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optNoTenantBoundErrorHandler{handler: handler}
}

type optObtainConnectionErrorHandler struct{ handler ErrorHandler }

func (o *optObtainConnectionErrorHandler) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.handleObtainConnectionError = o.handler
}

func WithObtainConnectionErrorHandler(handler ErrorHandler) MiddlewareOption {
	return &optObtainConnectionErrorHandler{handler: handler}
}