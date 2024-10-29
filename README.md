![CI][ci-status]
[![PkgGoDev][pkg-go-dev-badge]][pkg-go-dev]

# nagaya

Nagaya provides database multi-tenancy utility for Go web applications.

It is highly inspired by [apartment][], so you can use nagaya like apartment.

**Nagaya** is a style of apartments which typical for Edo period in Japan.

## Install

```sh
go get github.com/aereal/nagaya
```

## Synopsis

```go
import (
  "database/sql"
  "net/http"

  "github.com/aereal/nagaya"
)

func _() {
  var db *sql.DB
  manager := nagaya.NewStd(db)
  yourHandler := http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
    // obtain a database connection which bound for the current tenant.
    conn, _ := manager.ObtainConnection(r.Context())
    // run queries against the current tenant.
    _, _ = conn.GetContext(r.Context(), "select * from users")
  })
  // Nagaya middleware sets current tenant to the request context.
  // So you can obtain database connection bound for the current tenant in your HTTP handler via the context.
  mw := nagaya.Middleware(
    manager,
    nagaya.GetTenantFromHeader("tenant-id"), // this option tells the Nagaya uses `tenant-id` request header value as the current tenant
  )
  _ = mw(yourHandler)
}
```

## Developemnt and testing

```sh
docker compose up -d
port="$(docker compose ps --format json | jq '[(.Publishers[] | select(.TargetPort == 3306))][0].PublishedPort')"
export TEST_DB_DSN="root@tcp(127.0.0.1:${port})/tenant_default"
```

## License

See LICENSE file.

[pkg-go-dev]: https://pkg.go.dev/github.com/aereal/nagaya
[pkg-go-dev-badge]: https://pkg.go.dev/badge/aereal/nagaya
[ci-status]: https://github.com/aereal/nagaya/workflows/CI/badge.svg?branch=main
[apartment]: https://github.com/influitive/apartment
