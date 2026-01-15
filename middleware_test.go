package nagaya_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aereal/nagaya"
	_ "github.com/go-sql-driver/mysql"
)

const tenantHeaderDefault = "default"

func obo(headerName string) nagaya.DecideTenantFn {
	return func(r *http.Request) nagaya.TenantDecisionResult {
		tenant := r.Header.Get(headerName)
		switch tenant {
		case tenantHeaderDefault:
			return nagaya.TenantDecisionResultNoChange{}
		case "":
			return &nagaya.TenantDecisionResultError{Err: nagaya.ErrNoTenantBound}
		default:
			return &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant(tenant)}
		}
	}
}

func TestMiddleware(t *testing.T) {
	t.Parallel()
	ngy, err := newMySQLNagayaForTesting()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name                    string
		wantErrorMessage        string
		tenantIDHeader          string
		wantTenantFromContext   nagaya.Tenant
		options                 []nagaya.MiddlewareOption
		wantStatus              int
		wantTenantOKFromContext bool
	}{
		{
			name:                    "ok",
			wantStatus:              http.StatusOK,
			options:                 []nagaya.MiddlewareOption{nagaya.DecideTenantFromHeader("tenant-id")},
			tenantIDHeader:          "tenant_1",
			wantTenantFromContext:   "tenant_1",
			wantTenantOKFromContext: true,
		},
		{
			name:                    "ok/no tenant change",
			wantStatus:              http.StatusOK,
			options:                 []nagaya.MiddlewareOption{nagaya.WithDecideTenantFn(obo("tenant-id"))},
			tenantIDHeader:          tenantHeaderDefault,
			wantErrorMessage:        "no tenant change",
			wantTenantFromContext:   "",
			wantTenantOKFromContext: false,
		},
		{
			name:             "ng/no tenant id header",
			wantStatus:       http.StatusInternalServerError,
			wantErrorMessage: "no tenant bound for the context",
			options:          []nagaya.MiddlewareOption{nagaya.DecideTenantFromHeader("tenant-id")},
		},
		{
			name:             "ng/not configured how to get tenant",
			wantStatus:       http.StatusInternalServerError,
			wantErrorMessage: "no tenant bound for the context",
		},
		{
			name:             "ng/unknown tenant",
			wantStatus:       http.StatusInternalServerError,
			wantErrorMessage: "failed to change tenant to tenant_non_existent: Error 1049 (42000): Unknown database 'tenant_non_existent'",
			options:          []nagaya.MiddlewareOption{nagaya.DecideTenantFromHeader("tenant-id")},
			tenantIDHeader:   "tenant_non_existent",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				gotTenant, gotFoundTenant := nagaya.TenantFromContext(ctx)
				if tc.wantTenantFromContext != gotTenant {
					t.Errorf("TenantFromContext.tenant:\n\twant: %s\n\t got: %s", tc.wantTenantFromContext, gotTenant)
				}
				if tc.wantTenantOKFromContext != gotFoundTenant {
					t.Errorf("TenantFromContext.ok:\n\twant: %v\n\t got: %v", tc.wantTenantOKFromContext, gotFoundTenant)
				}
				if !gotFoundTenant {
					w.Header().Set("content-type", "application/json")
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, `{"error":"no tenant change"}`)
					return
				}
				conn, err := ngy.ObtainConnection(ctx)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if _, err := conn.ExecContext(ctx, "insert users values ()"); err != nil {
					http.Error(w, fmt.Sprintf("failed to insert user record: %s", err), http.StatusInternalServerError)
					return
				}
				rows, err := conn.QueryContext(ctx, "select * from users order by id desc limit 1")
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to select user: %s", err), http.StatusInternalServerError)
					return
				}
				defer func() { _ = rows.Close() }()
				type user struct{ UserID uint64 }
				var result struct{ Users []user }
				for rows.Next() {
					var u user
					if err := rows.Scan(&u.UserID); err != nil {
						http.Error(w, fmt.Sprintf("failed to scan result: %s", err), http.StatusInternalServerError)
						return
					}
					result.Users = append(result.Users, u)
				}
				_ = json.NewEncoder(w).Encode(result) //nolint:errcheck,errchkjson
			})
			srv := httptest.NewServer(nagaya.Middleware(ngy, tc.options...)(handler))
			t.Cleanup(func() { srv.Close() })

			ctx, cancel := context.WithCancel(context.Background())
			if deadline, ok := t.Deadline(); ok {
				ctx, cancel = context.WithDeadline(ctx, deadline)
			}
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext: %s", err)
			}
			if tc.tenantIDHeader != "" {
				req.Header.Set("tenant-id", tc.tenantIDHeader)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("http.Client.Do: %s", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body) //nolint:errcheck
				t.Errorf("failed to request: status=%d body=%s", resp.StatusCode, string(body))
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %s", err)
			}
			if body.Error != tc.wantErrorMessage {
				t.Errorf("error message:\n\twant: %q\n\tgot: %q", tc.wantErrorMessage, body.Error)
			}
		})
	}
}

func TestMiddleware_not_configured(t *testing.T) {
	ngy, err := newMySQLNagayaForTesting()
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		_, err := ngy.ObtainConnection(ctx)
		if err == nil {
			t.Errorf("expected no connection returned but expectedly got")
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	if deadline, ok := t.Deadline(); ok {
		ctx, cancel = context.WithDeadline(ctx, deadline)
	}
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext: %s", err)
	}
	req.Header.Set("tenant-id", "tenant_1")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("http.Client.Do: %s", err)
	}
	defer resp.Body.Close()
}

const envTestDBDSN = "TEST_DB_DSN"

var errDSNRequired = fmt.Errorf("%s is required", envTestDBDSN)

func newMySQLNagayaForTesting() (*nagaya.Nagaya[*sql.DB, *sql.Conn], error) {
	dsn := os.Getenv(envTestDBDSN)
	if dsn == "" {
		return nil, errDSNRequired
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return nagaya.NewStd(db), nil
}
