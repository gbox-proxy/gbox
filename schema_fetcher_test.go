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
	sh := schemaChangedHandler(func(oldDocument, newDocument *ast.Document, oldSchema, newSchema *graphql.Schema) {
		s.Require().NotNil(newDocument)
		s.Require().NotNil(newSchema)
	})
	i := caddy.Duration(time.Millisecond * 100)
	c := &Caching{}
	s.Require().NoError(c.Provision(caddy.Context{}))

	testCases := []struct {
		name             string
		upstream         string
		interval         *caddy.Duration
		expectedErrorMsg string
		onSchemaChange   schemaChangedHandler
		caching          *Caching
	}{
		{
			name:           "disabled_interval",
			upstream:       "http://localhost:9091",
			onSchemaChange: sh,
		},
		{
			name:           "with_caching",
			upstream:       "http://localhost:9091",
			onSchemaChange: sh,
			caching:        c,
		},
		{
			name:           "interval",
			upstream:       "http://localhost:9091",
			onSchemaChange: sh,
			interval:       &i,
		},
		{
			name:             "connection_refused",
			upstream:         "http://localhost:9092",
			expectedErrorMsg: "connection refused",
		},
	}

	for _, testCase := range testCases {
		ctx, cancel := context.WithCancel(context.Background())

		f := &schemaFetcher{
			context:         ctx,
			upstream:        testCase.upstream,
			timeout:         caddy.Duration(time.Millisecond * 50),
			onSchemaChanged: testCase.onSchemaChange,
			interval:        testCase.interval,
			header:          make(http.Header, 0),
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
		s.Require().NotNil(f.schema, "case %s: schema should not be nil", testCase.name)

		if testCase.interval != nil {
			f.schema = nil
			<-time.After(time.Duration(*testCase.interval) + time.Millisecond*100)

			s.Require().NotNil(f.schema, "case %s: schema should not be nil", testCase.name)
		}

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
