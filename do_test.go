package nagaya_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/aereal/nagaya"
)

func TestDo(t *testing.T) {
	t.Parallel()

	ngy, err := newMySQLNagayaForTesting()
	if err != nil {
		t.Fatal(err)
	}

	var count int
	handler := func(ctx context.Context) error {
		count++
		dbName, err := getCurrentDBName(ctx, ngy)
		if err != nil {
			return err
		}
		if dbName != "tenant_2" {
			return fmt.Errorf("unexpected db: %s", dbName) //nolint:err113 // ignore for test
		}
		return nil
	}
	decision := &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant("tenant_2")}
	if err := nagaya.Do(t.Context(), ngy, handler, nagaya.WithTenantDecisionResult(decision)); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the handler must be called once but %d times called", count)
	}
}

func TestYield(t *testing.T) {
	t.Parallel()

	ngy, err := newMySQLNagayaForTesting()
	if err != nil {
		t.Fatal(err)
	}

	handler := func(ctx context.Context) (string, error) {
		return getCurrentDBName(ctx, ngy)
	}
	decision := &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant("tenant_2")}
	dbName, err := nagaya.Yield(t.Context(), ngy, handler, nagaya.WithTenantDecisionResult(decision))
	if err != nil {
		t.Fatal(err)
	}
	if dbName != "tenant_2" {
		t.Errorf("unexpected DB: %s", dbName)
	}
}

func getCurrentDBName(ctx context.Context, ngy *nagaya.Nagaya[*sql.DB, *sql.Conn]) (string, error) {
	conn, err := ngy.ObtainConnection(ctx)
	if err != nil {
		return "", err
	}
	rows, err := conn.QueryContext(ctx, `select database()`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var dbName string
	_ = rows.Next()
	if err := rows.Scan(&dbName); err != nil {
		return "", err
	}
	return dbName, nil
}
