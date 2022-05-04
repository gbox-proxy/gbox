package gbox

import (
	"bytes"
	"net/http"
	"strings"
	"sync"

	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func writeResponseErrors(errors error, w http.ResponseWriter) error {
	gqlErrors := graphql.RequestErrorsFromError(errors)
	w.Header().Set("Content-Type", "application/json")

	if _, err := gqlErrors.WriteResponse(w); err != nil {
		return err
	}

	return nil
}

func normalizeGraphqlRequest(schema *graphql.Schema, gqlRequest *graphql.Request) error {
	if result, _ := gqlRequest.Normalize(schema); !result.Successful {
		return result.Errors
	}

	operation, _ := astparser.ParseGraphqlDocumentString(gqlRequest.Query)
	numOfOperations := operation.NumOfOperationDefinitions()
	operationName := strings.TrimSpace(gqlRequest.OperationName)
	report := &operationreport.Report{}

	if operationName == "" && numOfOperations > 1 {
		report.AddExternalError(operationreport.ErrRequiredOperationNameIsMissing())

		return report
	}

	if operationName == "" && numOfOperations == 1 {
		operationName = operation.OperationDefinitionNameString(0)
	}

	if !operation.OperationNameExists(operationName) {
		report.AddExternalError(operationreport.ErrOperationWithProvidedOperationNameNotFound(operationName))

		return report
	}

	gqlRequest.OperationName = operationName

	return nil
}
