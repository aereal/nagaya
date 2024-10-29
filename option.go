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
type MiddlewareOption func(cfg *middlewareConfig)

// WithTimeout configures the time a middleware waits the change of tenant.
func WithTimeout(dur time.Duration) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.changeTenantTimeout = dur }
}

func GetTenantFromHeader(headerName string) MiddlewareOption {
	return func(cfg *middlewareConfig) {
		cfg.getTenant = func(r *http.Request) (Tenant, bool) {
			v := r.Header.Get(headerName)
			if v == "" {
				return Tenant(""), false
			}
			return Tenant(v), true
		}
	}
}

func WithGetTenantFn(fn func(r *http.Request) (Tenant, bool)) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.getTenant = fn }
}

func WithRequestIDGenerator(gen RequestIDGenerator) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.reqIDGen = gen }
}

func WithErrorHandler(handler ErrorHandler) MiddlewareOption {
	return func(cfg *middlewareConfig) {
		cfg.handleChangeTenantError = handler
		cfg.handleGenerateRequestIDError = handler
		cfg.handleNoTenantBoundError = handler
		cfg.handleObtainConnectionError = handler
	}
}

func WithChangeTenantErrorHandler(handler ErrorHandler) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.handleChangeTenantError = handler }
}

func WithGenerateRequestIDErrorHandler(handler ErrorHandler) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.handleGenerateRequestIDError = handler }
}

func WithNoTenantBoundErrorHandler(handler ErrorHandler) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.handleNoTenantBoundError = handler }
}

func WithObtainConnectionErrorHandler(handler ErrorHandler) MiddlewareOption {
	return func(cfg *middlewareConfig) { cfg.handleObtainConnectionError = handler }
}
