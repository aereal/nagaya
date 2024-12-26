package nagayagqlgen_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/aereal/nagaya"
	"github.com/aereal/nagaya/github.com/99designs/gqlgen/nagayagqlgen"
	"github.com/aereal/nagaya/nagayatesting"
	"github.com/google/go-cmp/cmp"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestExtension(t *testing.T) {
	ngy, err := nagayatesting.NewMySQLNagayaForTesting()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		tenantIDHeader string
		options        []nagayagqlgen.Option
		want           *graphql.Response
	}{
		{
			name: "ok",
			options: []nagayagqlgen.Option{
				nagayagqlgen.DecideTenantFromHeader(headerTenantID),
			},
			tenantIDHeader: "tenant_1",
			want:           okResp,
		},
		{
			name: "ok/no tenant change",
			options: []nagayagqlgen.Option{
				nagayagqlgen.WithTenantDecider(nagayagqlgen.TenantDeciderFunc(getFromHeaderOrDefault)),
			},
			tenantIDHeader: tenantDefault,
			want:           okResp,
		},
		{
			name: "ng/no tenant id header",
			options: []nagayagqlgen.Option{
				nagayagqlgen.DecideTenantFromHeader(headerTenantID),
			},
			want: &graphql.Response{
				Data: json.RawMessage(`null`),
				Errors: gqlerror.List{
					{
						Message: "no tenant bound for the context",
					},
				},
			},
		},
		{
			name: "ng/not configured how to get tenant",
			want: &graphql.Response{
				Data: json.RawMessage(`null`),
				Errors: gqlerror.List{
					{
						Message: nagaya.ErrNoTenantBound.Error(),
					},
				},
			},
		},
		{
			name:           "ng/unknown tenant",
			tenantIDHeader: "tenant_non_existent",
			options: []nagayagqlgen.Option{
				nagayagqlgen.DecideTenantFromHeader(headerTenantID),
			},
			want: &graphql.Response{
				Data: json.RawMessage(`null`),
				Errors: gqlerror.List{
					{
						Message: "failed to change tenant to tenant_non_existent: Error 1049 (42000): Unknown database 'tenant_non_existent'",
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			es := &graphql.ExecutableSchemaMock{
				SchemaFunc: func() *ast.Schema { return schema },
				ExecFunc:   execFunc,
			}
			h := handler.New(es)
			ext := nagayagqlgen.NewExtension(ngy, tc.options...)
			h.Use(ext)
			h.AddTransport(transport.POST{})
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)

			ctx, cancel := context.WithCancel(context.Background())
			if deadline, ok := t.Deadline(); ok {
				ctx, cancel = context.WithDeadline(ctx, deadline)
			}
			defer cancel()
			gotResp, err := doRequest(ctx, srv.URL, tc.tenantIDHeader)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmpResponse(tc.want, gotResp); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}

func getFromHeader(opCtx *graphql.OperationContext) nagaya.TenantDecisionResult {
	v := opCtx.Headers.Get(headerTenantID)
	if v == "" {
		return &nagaya.TenantDecisionResultError{Err: nagaya.ErrNoTenantBound}
	}
	return &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant(v)}
}

func getFromHeaderOrDefault(opCtx *graphql.OperationContext) nagaya.TenantDecisionResult {
	switch v := opCtx.Headers.Get(headerTenantID); v {
	case tenantDefault:
		return nagaya.TenantDecisionResultNoChange{}
	case "":
		return &nagaya.TenantDecisionResultError{Err: nagaya.ErrNoTenantBound}
	default:
		return &nagaya.TenantDecisionResultChangeTenant{Tenant: nagaya.Tenant(v)}
	}
}

func doRequest(ctx context.Context, url string, tenantID string) (*graphql.Response, error) {
	// currently transport.POST does not propagate request headers to graphql.RawParams
	params := cloneParams(baseParams)
	if params.Headers == nil {
		params.Headers = make(http.Header)
	}
	params.Headers.Set("content-type", "application/json")
	if tenantID != "" {
		params.Headers.Set(headerTenantID, tenantID)
	}
	headers := params.Headers.Clone()
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(params); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return nil, err
	}
	for k, vs := range headers {
		k := k
		vs := vs
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	gqlResp := new(graphql.Response)
	if err := json.NewDecoder(resp.Body).Decode(gqlResp); err != nil {
		return nil, err
	}
	return gqlResp, nil
}

func execFunc(ctx context.Context) graphql.ResponseHandler {
	return func(ctx context.Context) *graphql.Response {
		oc := graphql.GetOperationContext(ctx)
		if oc == nil {
			return graphql.ErrorResponse(ctx, "no operation context bound")
		}
		id, ok := oc.Variables["id"].(int64)
		if !ok {
			return graphql.ErrorResponse(ctx, "no id variable")
		}
		buf := new(bytes.Buffer)
		user := map[string]any{
			"user": map[string]any{
				"id":   id,
				"name": fmt.Sprintf("user_%d", id),
			},
		}
		if err := json.NewEncoder(buf).Encode(user); err != nil {
			return graphql.ErrorResponse(ctx, err.Error())
		}
		return &graphql.Response{Data: buf.Bytes()}
	}
}

func cmpResponse(want, got *graphql.Response) string {
	return cmp.Diff(want, got, transformJsonRawMessage)
}

func cloneParams(p *graphql.RawParams) *graphql.RawParams {
	ret := new(graphql.RawParams)
	ret.Query = strings.Clone(p.Query)
	ret.OperationName = strings.Clone(p.OperationName)
	ret.Headers = p.Headers.Clone()
	ret.ReadTime = p.ReadTime
	if len(p.Variables) > 0 {
		ret.Variables = maps.Clone(p.Variables)
	}
	if len(p.Extensions) > 0 {
		ret.Extensions = maps.Clone(p.Extensions)
	}
	return ret
}

var (
	transformJsonRawMessage = cmp.Transformer("json.RawMessage", func(msg json.RawMessage) any {
		var v any
		if err := json.Unmarshal(msg, &v); err != nil {
			panic(err)
		}
		return v
	})
	rawSchema = `
	type User {
		name: String!
		id: Int!
	}
	type Query {
		user(id: Int!): User
	}
	`
	schema         = gqlparser.MustLoadSchema(&ast.Source{Input: rawSchema})
	headerTenantID = "tenant-id"
	tenantDefault  = "default"
	baseParams     = &graphql.RawParams{
		OperationName: "getUser",
		Query:         `query getUser($id: Int!) { user(id: $id) { id name } }`,
		Variables:     map[string]any{"id": int64(123)},
	}
	okResp = &graphql.Response{
		Data: json.RawMessage(`{"user":{"id":123,"name":"user_123"}}`),
	}
)
