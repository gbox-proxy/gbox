package gbox

import (
	"bytes"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"net/http"
	"sync"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

func writeResponseErrors(errors error, w http.ResponseWriter) error {
	gqlErrors := graphql.RequestErrorsFromError(errors)
	w.Header().Set("Content-Type", "application/json")

	if _, err := gqlErrors.WriteResponse(w); err != nil {
		return err
	}

	return nil
}
