package nagaya

import (
	"errors"
	"fmt"
)

var (
	// ErrNoTenantBound is an error represents no tenant bound for the context.
	ErrNoTenantBound = errors.New("no tenant bound for the context")
	// ErrNoConnectionBound is an error represents no DB connection obtained for the context.
	ErrNoConnectionBound = errors.New("no DB connection bound for the context")
)

// ObtainConnectionError is an error type represents the failure of obtaining DB connection.
type ObtainConnectionError struct {
	err error
}

func (e *ObtainConnectionError) Error() string {
	return fmt.Sprintf("failed to obtain connection: %s", e.err)
}

func (e *ObtainConnectionError) Unwrap() error {
	return e.err
}

// ChangeTenantError is an error type represents the failure of changing current tenant.
type ChangeTenantError struct {
	err    error
	tenant Tenant
}

func (e *ChangeTenantError) Error() string {
	return fmt.Sprintf("failed to change tenant to %s: %s", e.tenant, e.err)
}

func (e *ChangeTenantError) Unwrap() error {
	return e.err
}

// Tenant returns a tenant to be switched.
func (e *ChangeTenantError) Tenant() Tenant { return e.tenant }

// GenerateRequestIDError is an error type represents the failure of generating ID of the current request.
type GenerateRequestIDError struct {
	err error
}

func (e *GenerateRequestIDError) Error() string {
	return fmt.Sprintf("failed to generate request ID: %s", e.err)
}

func (e *GenerateRequestIDError) Unwrap() error { return e.err }
