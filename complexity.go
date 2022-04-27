package gbox

import (
	"fmt"

	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
)

type Complexity struct {
	// Max query depth accept, disabled by default.
	MaxDepth int `json:"max_depth,omitempty"`

	// Query node count limit, disabled by default.
	NodeCountLimit int `json:"node_count_limit,omitempty"`

	// Max query complexity, disabled by default.
	MaxComplexity int `json:"complexity,omitempty"`
}

func (c *Complexity) validateRequest(s *graphql.Schema, r *graphql.Request) (requestErrors graphql.RequestErrors) {
	result, err := r.CalculateComplexity(graphql.DefaultComplexityCalculator, s)

	if err != nil {
		requestErrors = graphql.RequestErrorsFromError(err)

		return requestErrors
	}

	if c.MaxDepth > 0 && result.Depth > c.MaxDepth {
		requestErrors = append(requestErrors, graphql.RequestError{Message: fmt.Sprintf("query max depth is %d, current %d", c.MaxDepth, result.Depth)})
	}

	if c.NodeCountLimit > 0 && result.NodeCount > c.NodeCountLimit {
		requestErrors = append(requestErrors, graphql.RequestError{Message: fmt.Sprintf("query node count limit is %d, current %d", c.NodeCountLimit, result.NodeCount)})
	}

	if c.MaxComplexity > 0 && result.Complexity > c.MaxComplexity {
		requestErrors = append(requestErrors, graphql.RequestError{Message: fmt.Sprintf("max query complexity allow is %d, current %d", c.MaxComplexity, result.Complexity)})
	}

	return requestErrors
}
