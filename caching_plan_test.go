package gbox

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/stretchr/testify/require"
)

func TestComputeCachingPlan(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store: s,
		Rules: map[string]*CachingRule{
			"rule1": {
				MaxAge: 3,
				Swr:    10,
			},
			"rule2": {
				MaxAge: 10,
				Swr:    3,
			},
			"rule3": {
				Types: map[string]graphql.RequestFields{
					"Book": {},
				},
				MaxAge: 1,
				Swr:    1,
			},
		},
	}
	cr := newTestCachingRequest()
	planner, _ := newCachingPlanner(cr, c)
	require.NotNil(t, planner)

	p, pErr := planner.getPlan()
	require.NoError(t, pErr)
	require.Equal(t, p.MaxAge, caddy.Duration(3))
	require.Equal(t, p.Swr, caddy.Duration(3))
}

func newTestCachingRequest() *cachingRequest {
	s, _ := graphql.NewSchemaFromString(`
type Query {
	users: [User!]!
}

type User {
	name: String!
}
`)
	s.Normalize()

	d, _ := astparser.ParseGraphqlDocumentBytes(s.Document())
	r, _ := http.NewRequest(
		"POST",
		"http://localhost:9090/graphql",
		strings.NewReader(`{"query": "query { users { name } }"}`),
	)
	gqlRequest := &graphql.Request{
		Query: `query GetUsers { users { name } }`,
	}
	gqlRequest.Normalize(s)

	cr := newCachingRequest(r, &d, s, gqlRequest)

	return cr
}
