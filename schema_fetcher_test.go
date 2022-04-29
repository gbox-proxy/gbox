package gbox

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/caddyserver/caddy/v2"
	"github.com/gbox-proxy/gbox/internal/testserver"
	"github.com/gbox-proxy/gbox/internal/testserver/generated"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type SchemaFetcherTestSuite struct {
	suite.Suite
}

func (s *SchemaFetcherTestSuite) TestProvision() {
	c := &Caching{}
	s.Require().NoError(c.Provision(caddy.Context{}))

	testCases := []struct {
		name             string
		upstream         string
		expectedErrorMsg string
		caching          *Caching
	}{
		{
			name:     "without_caching",
			upstream: "http://localhost:9091",
		},
		{
			name:     "with_caching",
			upstream: "http://localhost:9091",
			caching:  c,
		},
		{
			name:             "invalid_upstream",
			upstream:         "http://localhost:9092",
			expectedErrorMsg: "connection refused",
		},
	}

	for _, testCase := range testCases {
		sh := schemaChangedHandler(func(oldDocument, newDocument *ast.Document, oldSchema, newSchema *graphql.Schema) {
			s.Require().NotNilf(newDocument, "case %s: new document should not be nil", testCase.name)
			s.Require().NotNil(newSchema, "case %s: new schema should not be nil", testCase.name)
		})
		ctx, cancel := context.WithCancel(context.Background())
		f := &schemaFetcher{
			context:         ctx,
			upstream:        testCase.upstream,
			timeout:         caddy.Duration(time.Millisecond * 50),
			onSchemaChanged: sh,
			header:          make(http.Header),
			caching:         c,
			logger:          zap.NewNop(),
		}

		e := f.Provision(caddy.Context{})

		if testCase.expectedErrorMsg != "" {
			s.Require().Errorf(e, "case %s: should error", testCase.name)
			s.Require().Containsf(e.Error(), testCase.expectedErrorMsg, "case %s: unexpected error message", testCase.name)
			cancel()

			return
		}

		s.Require().NoErrorf(e, "case %s: should not error", testCase.name)
		cancel()
	}
}

func TestSchemaFetcher(t *testing.T) {
	h := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &testserver.Resolver{}}))
	s := &http.Server{
		Addr:    "localhost:9091",
		Handler: h,
	}
	defer s.Shutdown(context.Background())

	go func() {
		s.ListenAndServe()
	}()

	<-time.After(time.Millisecond * 10)

	suite.Run(t, new(SchemaFetcherTestSuite))
}
