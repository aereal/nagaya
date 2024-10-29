package nagaya

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/xid"
)

var defaultChangeTenantTimeout = time.Second * 5

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

func failsToDetermineTenant(_ *http.Request) (_ Tenant, _ bool) { return }

// Middleware returns a middleware function that determines target tenant and obtain the database connection against the tenant.
//
// The consumer must get the obtained connection via Nagaya.ObtainConnection method and use it to access the database.
func Middleware[DB DBish, Conn Connish](n *Nagaya[DB, Conn], opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := new(middlewareConfig)
	for _, o := range opts {
		o(cfg)
	}
	if cfg.changeTenantTimeout == 0 {
		cfg.changeTenantTimeout = defaultChangeTenantTimeout
	}
	if cfg.getTenant == nil {
		cfg.getTenant = failsToDetermineTenant
	}
	if cfg.reqIDGen == nil {
		cfg.reqIDGen = defaultIDGenerator
	}
	if cfg.handleChangeTenantError == nil {
		cfg.handleChangeTenantError = jsonErrorHandler
	}
	if cfg.handleNoTenantBoundError == nil {
		cfg.handleNoTenantBoundError = jsonErrorHandler
	}
	if cfg.handleObtainConnectionError == nil {
		cfg.handleObtainConnectionError = jsonErrorHandler
	}
	if cfg.handleGenerateRequestIDError == nil {
		cfg.handleGenerateRequestIDError = jsonErrorHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant, ok := cfg.getTenant(r)
			if !ok {
				cfg.handleNoTenantBoundError(w, r, ErrNoTenantBound)
				return
			}
			ctx := WithTenant(r.Context(), tenant)
			conn, err := n.getConn(ctx, n.db)
			if err != nil {
				cfg.handleObtainConnectionError(w, r, &ObtainConnectionError{err: err})
				return
			}
			defer func() {
				_ = conn.Close()
			}()
			exCtx, cancel := context.WithTimeout(ctx, cfg.changeTenantTimeout)
			defer cancel()
			if _, err := conn.ExecContext(exCtx, fmt.Sprintf("use %s", tenant)); err != nil {
				cfg.handleChangeTenantError(w, r, &ChangeTenantError{err: err, tenant: tenant})
				return
			}
			reqID, err := cfg.reqIDGen.GenerateID(ctx, r)
			if err != nil {
				cfg.handleGenerateRequestIDError(w, r, &GenerateRequestIDError{err: err})
				return
			}
			n.mux.Lock()
			n.conns[reqID] = conn
			n.mux.Unlock()
			defer func() {
				n.mux.Lock()
				defer n.mux.Unlock()
				delete(n.conns, reqID)
			}()
			next.ServeHTTP(w, r.WithContext(contextWithReqID(ctx, reqID)))
		})
	}
}

type RequestIDGenerator interface {
	GenerateID(ctx context.Context, r *http.Request) (string, error)
}

type RequestIDGeneratorFunc func(ctx context.Context, r *http.Request) (string, error)

func (f RequestIDGeneratorFunc) GenerateID(ctx context.Context, r *http.Request) (string, error) {
	return f(ctx, r)
}

var defaultIDGenerator = RequestIDGeneratorFunc(func(_ context.Context, _ *http.Request) (string, error) { return xid.New().String(), nil })

type reqIDCtxKey struct{}

func contextWithReqID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDCtxKey{}, id)
}

func reqIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(reqIDCtxKey{}).(string)
	return id, ok
}

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
