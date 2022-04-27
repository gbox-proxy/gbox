package gbox

import (
	"testing"

	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/stretchr/testify/require"
)

func TestComplexity(t *testing.T) {
	testCases := map[string]struct {
		complexity         *Complexity
		expectedErrorCount int
	}{
		"disabled_all": {
			complexity: &Complexity{},
		},
		"invalid": {
			complexity: &Complexity{
				NodeCountLimit: 1,
				MaxDepth:       1,
				MaxComplexity:  1,
			},
			expectedErrorCount: 3,
		},
		"invalid_node_count_limit": {
			complexity: &Complexity{
				NodeCountLimit: 1,
			},
			expectedErrorCount: 1,
		},
		"invalid_max_depth": {
			complexity: &Complexity{
				MaxDepth: 1,
			},
			expectedErrorCount: 1,
		},
		"invalid_max_complexity": {
			complexity: &Complexity{
				MaxComplexity: 1,
			},
			expectedErrorCount: 1,
		},
	}

	s, _ := graphql.NewSchemaFromString(`
type Query {
	books: [Book!]!
}

type Book {
	id: ID!
	title: String!
	buyers: [User!]!
}

type User {
	id: ID!
	name: String!
}
`)
	gqlRequest := &graphql.Request{
		Query: `query GetBooks { 
	books {
		buyers { 
			id
			name 
		} 
	} 
}`,
	}
	s.Normalize()
	gqlRequest.Normalize(s)

	for name, testCase := range testCases {
		err := testCase.complexity.validateRequest(s, gqlRequest)

		require.Equalf(t, testCase.expectedErrorCount, err.Count(), "case %s: unexpected error count", name)
	}
}
