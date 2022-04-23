package gbox

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/gbox-proxy/gbox/internal/testserver"
	"github.com/gbox-proxy/gbox/internal/testserver/generated"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	pureCaddyfile = `
	{
		http_port     9090
		https_port    9443
	}
	localhost:9090 {
	}
`
	caddyfilePattern = `
	{
		http_port     9090
		https_port    9443
	}
	localhost:9090 {
		route {
			gbox {
				upstream http://localhost:9091
				%s
			}
		}
	}
`
)

type IntegrationTestSuite struct {
	suite.Suite
}

func (s *IntegrationTestSuite) TestComplexity() {
	testCases := map[string]struct {
		extraConfig  string
		payload      string
		expectedBody string
	}{
		"enabled": {
			extraConfig: `
complexity {
	max_depth 1
	node_count_limit 1
	max_complexity 1
}
`,
			payload:      `{"query": "query { users { books { title } } }"}`,
			expectedBody: `{"errors":[{"message":"query max depth is 1, current 3"},{"message":"query node count limit is 1, current 2"},{"message":"max query complexity allow is 1, current 2"}]}`,
		},
		"disabled": {
			extraConfig: `
complexity {
	enabled false
	max_depth 1
}
`,
			payload:      `{"query": "query { users { name } }"}`,
			expectedBody: `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
	}

	for name, testCase := range testCases {
		tester := caddytest.NewTester(s.T())
		tester.InitServer(pureCaddyfile, "caddyfile")
		tester.InitServer(fmt.Sprintf(caddyfilePattern, testCase.extraConfig), "caddyfile")

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(testCase.payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := ioutil.ReadAll(resp.Body)

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case: %s", name)
		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestIntrospection() {
	testCases := map[string]struct {
		extraConfig  string
		payload      string
		expectedBody string
	}{
		"enabled": {
			extraConfig:  "disabled_introspection false",
			payload:      `{"query": "query { __schema { queryType { name } } }"}`,
			expectedBody: `{"data":{"__schema":{"queryType":{"name":"QueryTest"}}}}`,
		},
		"disabled": {
			extraConfig:  "disabled_introspection true",
			payload:      `{"query": "query { __schema { queryType { name } } }"}`,
			expectedBody: `{"errors":[{"message":"introspection queries are not allowed"}]}`,
		},
	}

	for name, testCase := range testCases {
		tester := caddytest.NewTester(s.T())
		tester.InitServer(pureCaddyfile, "caddyfile")
		tester.InitServer(fmt.Sprintf(caddyfilePattern, testCase.extraConfig), "caddyfile")

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(testCase.payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case: %s", name)
		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestCachingPassAndMiss() {
	const payload = `{"query": "query { users { name } }"}`

	testCases := map[string]struct {
		extraConfig           string
		expectedCachingStatus CachingStatus
		expectedBody          string
	}{
		"pass_when_empty_rules": {
			extraConfig: `
caching {
}
		`,
			expectedCachingStatus: CachingStatusPass,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		"pass_when_not_match_type": {
			extraConfig: `
caching {
	rules {
		book {
			max_age 5m
			types {
				BookTest
			}
		}
	}
}
		`,
			expectedCachingStatus: CachingStatusPass,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		"pass_when_not_match_field": {
			extraConfig: `
caching {
	rules {
		user {
			max_age 5m
			types {
				UserTest id
			}
		}
	}
}
		`,
			expectedCachingStatus: CachingStatusPass,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		"miss_on_match_type": {
			extraConfig: `
caching {
	debug_headers true
	rules {
		user {
			max_age 5m
			types {
				UserTest
			}
		}
	}
}
`,
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		"miss_on_match_type_field": {
			extraConfig: `
caching {
	debug_headers true
	rules {
		user {
			max_age 5m
			types {
				UserTest name
			}
		}
	}
}
`,
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
	}

	for name, testCase := range testCases {
		tester := caddytest.NewTester(s.T())
		tester.InitServer(pureCaddyfile, "caddyfile")
		tester.InitServer(fmt.Sprintf(caddyfilePattern, testCase.extraConfig), "caddyfile")
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualStatus := resp.Header.Get("x-cache")

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", name)

		resp.Body.Close()
	}
}

func TestIntegration(t *testing.T) {
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

	suite.Run(t, new(IntegrationTestSuite))
}
