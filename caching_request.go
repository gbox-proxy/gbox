package gbox

import (
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astnormalization"
	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"github.com/pquerna/cachecontrol/cacheobject"
	"net/http"
	"strings"
)

type cachingRequest struct {
	operationName         string
	httpRequest           *http.Request
	schema                *graphql.Schema
	gqlRequest            *graphql.Request
	definition, operation *ast.Document
	cacheControl          *cacheobject.RequestCacheDirectives
}

func newCachingRequest(r *http.Request, d *ast.Document, s *graphql.Schema, gr *graphql.Request) (*cachingRequest, error) {
	cr := &cachingRequest{
		httpRequest: r,
		schema:      s,
		definition:  d,
		gqlRequest:  gr,
	}

	operation, report := astparser.ParseGraphqlDocumentString(gr.Query)

	if report.HasErrors() {
		return nil, &report
	}

	report.Reset()

	operation.Input.Variables = gr.Variables
	numOfOperations := operation.NumOfOperationDefinitions()
	operationName := strings.TrimSpace(gr.OperationName)

	if len(operationName) == 0 && numOfOperations > 1 {
		report.AddExternalError(operationreport.ErrRequiredOperationNameIsMissing())

		return nil, &report
	}

	if len(operationName) == 0 && numOfOperations == 1 {
		operationName = operation.OperationDefinitionNameString(0)
	}

	if !operation.OperationNameExists(operationName) {
		report.AddExternalError(operationreport.ErrOperationWithProvidedOperationNameNotFound(operationName))

		return nil, &report
	}

	report.Reset()

	normalizer := astnormalization.NewWithOpts(
		astnormalization.WithExtractVariables(),
		astnormalization.WithRemoveFragmentDefinitions(),
		astnormalization.WithRemoveUnusedVariables(),
	)

	if len(operationName) > 0 {
		normalizer.NormalizeNamedOperation(&operation, d, []byte(operationName), &report)
	} else {
		normalizer.NormalizeOperation(&operation, d, &report)
	}

	if report.HasErrors() {
		return nil, &report
	}

	cr.operationName = operationName
	cr.operation = &operation
	cacheControlString := r.Header.Get("cache-control")
	cr.cacheControl, _ = cacheobject.ParseRequestCacheControl(cacheControlString)

	return cr, nil
}
