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

func WithRequestIDGenerator(gen RequestIDGenerator) Option {
	return func(c *extensionConfig) { c.reqIDGen = gen }
}

func WithErrorHandler(h ErrorHandler) Option { return func(c *extensionConfig) { c.errorHandler = h } }

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *extensionConfig) { c.tp = tp }
}

// WithChangeTenantTimeout sets the how long wait for a tenant change.
func WithChangeTenantTimeout(timeout time.Duration) Option {
	return func(c *extensionConfig) { c.changeTenantTimeout = timeout }
}
