package gbox

import (
	"fmt"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

const (
	caddyfilePattern = `
	{
		http_port     9090
		https_port    9443
	}
	localhost:9090 {
		route {
			gbox {
				upstream https://countries.trevorblades.com
				%s
			}
		}
	}
`
)

type IntegrationTestSuite struct {
	suite.Suite
}

func (s *IntegrationTestSuite) TestDisabledIntrospectionAndComplexity() {
	testCases := map[string]struct {
		extraConfig  string
		expectedBody string
	}{
		"test_disabled_introspection": {
			extraConfig:  "disabled_introspection true",
			expectedBody: `{"errors":[{"message":"introspection queries are not allowed"}]}`,
		},
		"test_query_complexity": {
			extraConfig: `
complexity {
	max_depth 1
	node_count_limit 1
	max_complexity 1
}
`,
			expectedBody: `{"errors":[{"message":"query max depth is 1, current 3"},{"message":"query node count limit is 1, current 2"},{"message":"max query complexity allow is 1, current 2"}]}`,
		},
		"test_disabled_complexity": {
			extraConfig: `
complexity {
	enabled false
	max_depth 1
	node_count_limit 1
	max_complexity 1
}
`,
			expectedBody: `{"data":{"__schema":{"queryType":{"name":"Query"}}}}`,
		},
	}

	for name, testCase := range testCases {
		tester := caddytest.NewTester(s.T())
		tester.InitServer(fmt.Sprintf(caddyfilePattern, testCase.extraConfig), "caddyfile")

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query": "query { __schema { queryType { name } } }"}`),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := ioutil.ReadAll(resp.Body)
		actualBody := strings.TrimSpace(string(respBody))

		require.Equalf(s.T(), testCase.expectedBody, actualBody, "case: %s", name)
	}
}

func TestIntegration(t *testing.T) {
	s := new(IntegrationTestSuite)
	suite.Run(t, s)
}
