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

func (s *IntegrationTestSuite) TestCachingStatues() {
	const payload = `{"query": "query { users { name } }"}`

	testCases := []struct {
		name                  string
		extraConfig           string
		expectedCachingStatus CachingStatus
		expectedBody          string
		resetTester           bool
	}{
		{
			name: "pass_when_empty_rules",
			extraConfig: `
caching {
}
		`,
			expectedCachingStatus: CachingStatusPass,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			resetTester:           true,
		},
		{
			name: "pass_when_not_match_type",
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
			resetTester:           true,
		},
		{
			name: "pass_when_not_match_field",
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
			resetTester:           true,
		},
		{
			name: "miss_on_match_type",
			extraConfig: `
caching {
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
			resetTester:           true,
		},
		{
			name: "miss_on_first_time_match_type_field",
			extraConfig: `
caching {
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
			resetTester:           true,
		},
		{
			name: "hit_on_second_time_match_type_field",
			extraConfig: `
caching {
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
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			resetTester:           false,
		},
	}

	tester := caddytest.NewTester(s.T())

	for _, testCase := range testCases {
		if testCase.resetTester {
			tester.InitServer(pureCaddyfile, "caddyfile")
		}

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

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)

		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestCachingSwr() {
	testCases := []struct {
		name                  string
		expectedHitTimes      string
		expectedCachingStatus CachingStatus
		expectedBody          string
		expectedCachingTags   []string
		executeAfter          time.Duration
	}{
		{
			name:                  "miss_on_first_time",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		{
			name:                  "hit_on_next_time",
			executeAfter:          time.Millisecond, // wait for caching in background
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "1",
		},
		{
			name:                  "swr_after_60ms",
			executeAfter:          time.Millisecond * 60,
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "2",
		},
		{
			name:                  "result_revalidated_in_background",
			executeAfter:          time.Millisecond, // wait for revalidating in background
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "1", // hit times had been reset.
		},
	}
	const payload = `{"query": "query { users { name } }"}`
	const config = `
caching {
	rules {
		test {
			max_age 60ms
			swr 60ms
		}
	}
}
`
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, config), "caddyfile")

	for _, testCase := range testCases {
		<-time.After(testCase.executeAfter)

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualStatus := resp.Header.Get("x-cache")
		actualHitTimes := resp.Header.Get("x-cache-hits")

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		require.Equalf(s.T(), testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestCachingEnabledAutoInvalidate() {
	const payloadNameOnly = `{"query": "query UsersNameOnly { users { name } }"}`
	const payload = `{"query": "query Users { users { id name } }"}`
	const mutationPayload = `{"query": "mutation InvalidateUsers { updateUsers { id } }"}`
	testCases := []struct {
		name                  string
		expectedHitTimes      string
		expectedCachingStatus CachingStatus
		expectedBody          string
		expectedCachingTags   string
		expectedPurgingTags   string
		payload               string
	}{
		{
			name:                  "no_type_keys_miss_on_first_time",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			payload:               payloadNameOnly,
		},
		{
			name:                  "no_type_keys_hit_on_next_time",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "1",
			payload:               payloadNameOnly,
			expectedCachingTags:   `field:QueryTest:users, field:UserTest:name, operation:UsersNameOnly, schema:4230843191964202593, type:QueryTest, type:UserTest`,
		},
		{
			name:                  "type_keys_miss_on_first_time",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			payload:               payload,
		},
		{
			name:                  "type_keys_hit_on_next_time",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			expectedHitTimes:      "1",
			payload:               payload,
			expectedCachingTags:   `field:QueryTest:users, field:UserTest:id, field:UserTest:name, key:UserTest:id:1, key:UserTest:id:2, key:UserTest:id:3, operation:Users, schema:4230843191964202593, type:QueryTest, type:UserTest`,
		},
		{
			name:                "mutation_invalidate_query_result",
			expectedBody:        `{"data":{"updateUsers":[{"id":1},{"id":2}]}}`,
			expectedPurgingTags: `key:UserTest:id:1; key:UserTest:id:2`,
			payload:             mutationPayload,
		},
		{
			name:                  "mutation_invalidated_query_result",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			payload:               payload,
		},
		{
			name:                  "mutation_not_invalidated_query_result",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "2",
			payload:               payloadNameOnly,
			expectedCachingTags:   `field:QueryTest:users, field:UserTest:name, operation:UsersNameOnly, schema:4230843191964202593, type:QueryTest, type:UserTest`,
		},
	}

	const config = `
caching {
	auto_invalidate_cache true
	debug_headers true
	rules {
		test {
			max_age 1h
		}
	}
}
`
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, config), "caddyfile")

	for _, testCase := range testCases {
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(testCase.payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualHitTimes := resp.Header.Get("x-cache-hits")
		actualStatus := resp.Header.Get("x-cache")
		actualCachingTags := resp.Header.Get("x-debug-result-tags")
		actualPurgingTags := resp.Header.Get("x-debug-purging-tags")

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		require.Equalf(s.T(), testCase.expectedCachingTags, actualCachingTags, "case %s: unexpected caching tags", testCase.name)
		require.Equalf(s.T(), testCase.expectedPurgingTags, actualPurgingTags, "case %s: unexpected purging tags", testCase.name)
		require.Equalf(s.T(), testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestCachingDisabledAutoInvalidate() {
	const payload = `{"query": "query Users { users { id name } }"}`
	const mutationPayload = `{"query": "mutation InvalidateUsers { updateUsers { id } }"}`
	testCases := []struct {
		name                  string
		expectedHitTimes      string
		expectedCachingStatus CachingStatus
		expectedBody          string
		expectedPurgingTags   string
		payload               string
	}{
		{
			name:                  "miss_on_first_time",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			payload:               payload,
		},
		{
			name:                  "hit_on_next_time",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			expectedHitTimes:      "1",
			payload:               payload,
		},
		{
			name:                "mutation_invalidated_query_result_disabled",
			expectedPurgingTags: "",
			expectedBody:        `{"data":{"updateUsers":[{"id":1},{"id":2}]}}`,
			payload:             mutationPayload,
		},
		{
			name:                  "mutation_not_invalidate_query_result",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`,
			expectedHitTimes:      "2",
			payload:               payload,
		},
	}

	const config = `
caching {
	auto_invalidate_cache false
	debug_headers true
	rules {
		test {
			max_age 1h
		}
	}
}
`
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, config), "caddyfile")

	for _, testCase := range testCases {
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(testCase.payload),
		)
		r.Header.Add("content-type", "application/json")

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualHitTimes := resp.Header.Get("x-cache-hits")
		actualStatus := resp.Header.Get("x-cache")
		actualPurgingTags := resp.Header.Get("x-debug-purging-tags")

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		require.Equalf(s.T(), testCase.expectedPurgingTags, actualPurgingTags, "case %s: unexpected purging tags", testCase.name)
		require.Equalf(s.T(), testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) TestCachingVaries() {
	testCases := []struct {
		name                  string
		expectedHitTimes      string
		expectedCachingStatus CachingStatus
		expectedBody          string
		vary                  *struct {
			headers map[string]string
			cookies map[string]string
		}
	}{
		{
			name:                  "miss_on_first_time_without_vary",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
		},
		{
			name:                  "hit_on_next_time_without_vary",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			expectedHitTimes:      "1",
		},
		{
			name:                  "miss_on_difference_vary_headers",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				headers: map[string]string{
					"x-test": "1",
				},
			},
		},
		{
			name:                  "hit_on_same_vary_headers",
			expectedHitTimes:      "1",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				headers: map[string]string{
					"x-test": "1",
				},
			},
		},
		{
			name:                  "miss_on_difference_vary_cookies",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				cookies: map[string]string{
					"x-test": "1",
				},
			},
		},
		{
			name:                  "hit_on_same_vary_cookies",
			expectedHitTimes:      "1",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				cookies: map[string]string{
					"x-test": "1",
				},
			},
		},
		{
			name:                  "miss_on_difference_vary_headers_cookies",
			expectedCachingStatus: CachingStatusMiss,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				headers: map[string]string{
					"x-test": "1",
				},
				cookies: map[string]string{
					"x-test": "2",
				},
			},
		},
		{
			name:                  "hit_on_difference_vary_headers_cookies",
			expectedHitTimes:      "1",
			expectedCachingStatus: CachingStatusHit,
			expectedBody:          `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`,
			vary: &struct {
				headers map[string]string
				cookies map[string]string
			}{
				headers: map[string]string{
					"x-test": "1",
				},
				cookies: map[string]string{
					"x-test": "2",
				},
			},
		},
	}

	const payload = `{"query": "query { users { name } }"}`
	const config = `
caching {
	rules {
		test {
			max_age 60ms
			swr 60ms
			varies test
		}
	}
	varies {
		test {
			headers x-test
			cookies x-test
		}
	}
}
`
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, config), "caddyfile")

	for _, testCase := range testCases {
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(payload),
		)
		r.Header.Add("content-type", "application/json")

		if testCase.vary != nil {
			for h, v := range testCase.vary.headers {
				r.Header.Set(h, v)
			}

			for c, v := range testCase.vary.cookies {
				r.AddCookie(&http.Cookie{Name: c, Value: v})
			}
		}

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualStatus := resp.Header.Get("x-cache")
		actualHitTimes := resp.Header.Get("x-cache-hits")

		require.Equalf(s.T(), testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		require.Equalf(s.T(), string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		require.Equalf(s.T(), testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

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
