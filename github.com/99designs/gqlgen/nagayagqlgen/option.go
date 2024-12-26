package nagayagqlgen

import (
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/nagaya"
	"go.opentelemetry.io/otel/trace"
)

type extensionConfig struct {
	changeTenantTimeout time.Duration
	tenantDecider       TenantDecider
	reqIDGen            RequestIDGenerator
	errorHandler        ErrorHandler
	tp                  trace.TracerProvider
}

type Option func(c *extensionConfig)

// DecideTenantFromHeader tells the extension to use given header value to decide the tenant.
func DecideTenantFromHeader(headerName string) Option {
	return func(c *extensionConfig) {
		c.tenantDecider = TenantDeciderFunc(func(opCtx *graphql.OperationContext) nagaya.TenantDecisionResult {
			v := opCtx.Headers.Get(headerName)
			if v == "" {
				return &nagaya.TenantDecisionResultError{Err: nagaya.ErrNoTenantBound}
			}
			return &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant(v)}
		})
	}
}

// WithTenantDecider tells the extension to use given function to decide the tenant.
func WithTenantDecider(fn TenantDecider) Option {
	return func(c *extensionConfig) {
		c.tenantDecider = fn
	}
}

// WithRequestIDGenerator tells the extensino to use given [RequestIDGenerator].
func WithRequestIDGenerator(gen RequestIDGenerator) Option {
	return func(c *extensionConfig) { c.reqIDGen = gen }
}

// WithErrorHandler tells the extension to use given error handler.
func WithErrorHandler(h ErrorHandler) Option { return func(c *extensionConfig) { c.errorHandler = h } }

// WithTracerProvider tells the extension to use given [trace.TracerProvider] to build tracer.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *extensionConfig) { c.tp = tp }
}

// WithChangeTenantTimeout sets the how long wait for a tenant change.
func WithChangeTenantTimeout(timeout time.Duration) Option {
	return func(c *extensionConfig) { c.changeTenantTimeout = timeout }
}
