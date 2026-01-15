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
	decideTenant      DecideRequestTenantFunc
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

type doConfig struct {
	reqIDGen           RequestIDGenerator
	tenantDecisionRet  TenantDecisionResult
	bindConnectionOpts []BindConnectionOption
}

type DoOption interface {
	applyDoOption(c *doConfig)
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

func (o *optTimeout) applyDoOption(c *doConfig) {
	c.bindConnectionOpts = append(c.bindConnectionOpts, o)
}

// WithTimeout sets the how long wait for a tenant change.
func WithTimeout(dur time.Duration) interface {
	MiddlewareOption
	BindConnectionOption
	DoOption
} {
	return &optTimeout{dur: dur}
}

type optDecideTenantFn struct {
	fn DecideRequestTenantFunc
}

func (o *optDecideTenantFn) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.decideTenant = o.fn
}

// WithDecideTenantFn tells the middleware to use given function to decide the tenant.
func WithDecideTenantFn(fn DecideRequestTenantFunc) MiddlewareOption {
	return &optDecideTenantFn{fn: fn}
}

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

func (o *optRequestIDGenerator) applyDoOption(c *doConfig) { c.reqIDGen = o.gen }

// WithRequestIDGenerator tells the middleware to use given [RequestIDGenerator].
func WithRequestIDGenerator(gen RequestIDGenerator) interface {
	MiddlewareOption
	DoOption
} {
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

func WithTenantDecisionResult(r TenantDecisionResult) DoOption { return &optTenantDecisionResult{r} }

type optTenantDecisionResult struct{ TenantDecisionResult }

func (o *optTenantDecisionResult) applyDoOption(c *doConfig) {
	c.tenantDecisionRet = o.TenantDecisionResult
}
