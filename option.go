package nagaya

import (
	"net/http"
	"time"
)

type middlewareConfig struct {
	reqIDGen                     RequestIDGenerator
	getTenant                    func(r *http.Request) (Tenant, bool)
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

type optGetTenantFromHeader struct{ headerName string }

func (o *optGetTenantFromHeader) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.getTenant = func(r *http.Request) (Tenant, bool) {
		v := r.Header.Get(o.headerName)
		if v == "" {
			return Tenant(""), false
		}
		return Tenant(v), true
	}
}

func GetTenantFromHeader(headerName string) MiddlewareOption {
	return &optGetTenantFromHeader{headerName: headerName}
}

type optGetTenantFn struct {
	fn func(*http.Request) (Tenant, bool)
}

func (o *optGetTenantFn) applyMiddlewareOption(cfg *middlewareConfig) {
	cfg.getTenant = o.fn
}

func WithGetTenantFn(fn func(r *http.Request) (Tenant, bool)) MiddlewareOption {
	return &optGetTenantFn{fn: fn}
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
