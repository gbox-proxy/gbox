package gbox

import (
	"net/http"

	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astnormalization"
	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/pquerna/cachecontrol/cacheobject"
)

type cachingRequest struct {
	httpRequest           *http.Request
	schema                *graphql.Schema
	gqlRequest            *graphql.Request
	definition, operation *ast.Document
	cacheControl          *cacheobject.RequestCacheDirectives
}

func newCachingRequest(r *http.Request, d *ast.Document, s *graphql.Schema, gr *graphql.Request) *cachingRequest {
	cr := &cachingRequest{
		httpRequest: r,
		schema:      s,
		definition:  d,
		gqlRequest:  gr,
	}

	cacheControlString := r.Header.Get("cache-control")
	cr.cacheControl, _ = cacheobject.ParseRequestCacheControl(cacheControlString)

	return cr
}

func (r *cachingRequest) initOperation() error {
	if r.operation != nil {
		return nil
	}

	operation, report := astparser.ParseGraphqlDocumentString(r.gqlRequest.Query)

	if report.HasErrors() {
		return &report
	}

	operation.Input.Variables = r.gqlRequest.Variables
	normalizer := astnormalization.NewWithOpts(
		astnormalization.WithExtractVariables(),
		astnormalization.WithRemoveFragmentDefinitions(),
		astnormalization.WithRemoveUnusedVariables(),
	)

	if r.gqlRequest.OperationName != "" {
		normalizer.NormalizeNamedOperation(&operation, r.definition, []byte(r.gqlRequest.OperationName), &report)
	} else {
		normalizer.NormalizeOperation(&operation, r.definition, &report)
	}

	if report.HasErrors() {
		return &report
	}

	r.operation = &operation

	return nil
}
