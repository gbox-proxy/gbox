package gbox

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/gbox-proxy/gbox/internal/testserver"
	"github.com/gbox-proxy/gbox/internal/testserver/generated"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/suite"
)

const (
	pureCaddyfile = `
	{
		http_port     9090
		https_port    9443
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

type HandlerIntegrationTestSuite struct {
	suite.Suite
}

func (s *HandlerIntegrationTestSuite) TestComplexity() {
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case: %s", name)
		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestIntrospection() {
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case: %s", name)
		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestDisabledCaching() {
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
caching {
	enabled false
	rules {
		default {
			max_age 1h
		}
	}
}
`), "caddyfile")
	r, _ := http.NewRequest(
		"POST",
		"http://localhost:9090/graphql",
		strings.NewReader(`{"query": "query UsersNameOnly { users { name } }"}`),
	)
	r.Header.Add("content-type", "application/json")
	resp := tester.AssertResponseCode(r, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Emptyf(resp.Header.Get("x-cache"), "x-cache header should not be set")
	s.Equalf(`{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`, string(body), "unexpected response")
}

func (s *HandlerIntegrationTestSuite) TestNotCachingInvalidResponse() {
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
caching {
	rules {
		default {
			max_age 1h
		}
	}
}
`), "caddyfile")

	for i := 0; i < 3; i++ {
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query": "query UsersNameOnly { users(invalid_filter: 123) { name } }"}`),
		)
		r.Header.Add("content-type", "application/json")
		resp := tester.AssertResponseCode(r, http.StatusUnprocessableEntity)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		s.Equalf(string(CachingStatusMiss), resp.Header.Get("x-cache"), "cache status should be MISS at all")
		s.Equalf(`{"errors":[{"message":"Unknown argument \"invalid_filter\" on field \"QueryTest.users\".","locations":[{"line":1,"column":23}],"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`, string(body), "unexpected response")
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingStatues() {
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingSwr() {
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
			executeAfter:          time.Millisecond * 5, // wait for revalidating in background
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		s.Require().Equalf(testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingEnabledAutoInvalidate() {
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
		actualPurgingTags := resp.Header.Get("x-debug-purged-tags")

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		s.Require().Equalf(testCase.expectedCachingTags, actualCachingTags, "case %s: unexpected caching tags", testCase.name)
		s.Require().Equalf(testCase.expectedPurgingTags, actualPurgingTags, "case %s: unexpected purging tags", testCase.name)
		s.Require().Equalf(testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingDisabledAutoInvalidate() {
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		s.Require().Equalf(testCase.expectedPurgingTags, actualPurgingTags, "case %s: unexpected purging tags", testCase.name)
		s.Require().Equalf(testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingVaries() {
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

		s.Require().Equalf(testCase.expectedBody, string(respBody), "case %s: unexpected payload", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualStatus, "case %s: unexpected status", testCase.name)
		s.Require().Equalf(testCase.expectedHitTimes, actualHitTimes, "case %s: unexpected hit times", testCase.name)

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestCombinedCachingVaries() {
	const payload = `{"query": "query { users { name } }"}`
	const config = `
caching {
	rules {
		test {
			max_age 60ms
			swr 60ms
			varies first second
		}
	}
	varies {
		first {
			headers x-first-header-1 x-first-header-2
			cookies x-first-cookie-1 x-first-cookie-2
		}
		second {
			headers x-second-header-1 x-second-header-2
			cookies x-second-cookie-1 x-second-cookie-2
		}
	}
}
`
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, config), "caddyfile")

	// test cache hit for 20 times
	for i := 0; i <= 20; i++ {
		expectedCachingStatus := CachingStatusHit
		expectedHitTimes := strconv.Itoa(i)
		expectedBody := `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`
		if i == 0 {
			expectedCachingStatus = CachingStatusMiss
			expectedHitTimes = ""
		}

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(payload),
		)
		r.Header.Add("content-type", "application/json")

		r.Header.Set("x-first-header-1", "x-first-header-1")
		r.Header.Set("x-first-header-2", "x-first-header-2")
		r.Header.Set("x-second-header-1", "x-second-header-1")
		r.Header.Set("x-second-header-2", "x-second-header-2")

		r.AddCookie(&http.Cookie{Name: "x-first-cookie-1", Value: "x-first-cookie-1"})
		r.AddCookie(&http.Cookie{Name: "x-first-cookie-2", Value: "x-first-cookie-2"})
		r.AddCookie(&http.Cookie{Name: "x-second-cookie-1", Value: "x-second-cookie-1"})
		r.AddCookie(&http.Cookie{Name: "x-second-cookie-2", Value: "x-second-cookie-2"})

		resp := tester.AssertResponseCode(r, http.StatusOK)
		respBody, _ := io.ReadAll(resp.Body)
		actualStatus := resp.Header.Get("x-cache")
		actualHitTimes := resp.Header.Get("x-cache-hits")

		s.Require().Equalf(expectedBody, string(respBody), "unexpected payload")
		s.Require().Equalf(string(expectedCachingStatus), actualStatus, "unexpected status")
		s.Require().Equalf(expectedHitTimes, actualHitTimes, "unexpected hit times")

		resp.Body.Close()
	}
}

func (s *HandlerIntegrationTestSuite) TestEnabledPlaygrounds() {
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
disabled_playgrounds false
`), "caddyfile")
	r, _ := http.NewRequest("GET", "http://localhost:9090/", nil)
	resp := tester.AssertResponseCode(r, http.StatusOK)
	resp.Body.Close()

	r, _ = http.NewRequest("GET", "http://localhost:9090/admin", nil)
	resp = tester.AssertResponseCode(r, http.StatusNotFound) // when not enable caching, admin play ground should not affect.
	resp.Body.Close()

	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
disabled_playgrounds false
caching {
}
`), "caddyfile")

	r, _ = http.NewRequest("GET", "http://localhost:9090/admin", nil)
	resp = tester.AssertResponseCode(r, http.StatusOK) // now it should be enabled.
	resp.Body.Close()

	r, _ = http.NewRequest("GET", "http://localhost:9090", nil)
	resp = tester.AssertResponseCode(r, http.StatusOK)
	resp.Body.Close()
}

func (s *HandlerIntegrationTestSuite) TestDisabledPlaygrounds() {
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
disabled_playgrounds true
`), "caddyfile")
	r, _ := http.NewRequest("GET", "http://localhost:9090", nil)
	resp := tester.AssertResponseCode(r, http.StatusNotFound)
	resp.Body.Close()

	r, _ = http.NewRequest("GET", "http://localhost:9090/admin", nil)
	resp = tester.AssertResponseCode(r, http.StatusNotFound)
	resp.Body.Close()
}

func (s *HandlerIntegrationTestSuite) TestMetrics() {
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
caching {
	rules {
		default {
			types {
				UserTest
			}
			max_age 30m
		}
	}
}
`), "caddyfile")

	for i := 1; i <= 3; i++ {
		br, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query":"query GetBooksMetric { books { title } }"}`),
		)
		br.Header.Add("content-type", "application/json")
		resp := tester.AssertResponseCode(br, http.StatusOK)
		resp.Body.Close()

		ur, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query":"query GetUsersMetric { users { name } }"}`),
		)
		ur.Header.Add("content-type", "application/json")
		resp = tester.AssertResponseCode(ur, http.StatusOK)
		resp.Body.Close()

		var metricOut dto.Metric

		c, ce := metrics.operationCount.GetMetricWith(prometheus.Labels{
			"operation_name": "GetUsersMetric",
			"operation_type": "query",
		})

		s.Require().NoError(ce)
		s.Require().NoError(c.Write(&metricOut))
		s.Require().Equal(float64(i), *metricOut.Counter.Value, "unexpected operation count metrics")

		cm, cme := metrics.cachingCount.GetMetricWith(prometheus.Labels{
			"operation_name": "GetUsersMetric",
			"status":         string(CachingStatusMiss),
		})

		s.Require().NoError(cme)
		s.Require().NoError(cm.Write(&metricOut))
		s.Require().Equal(float64(1), *metricOut.Counter.Value, "unexpected cache miss metrics")

		ch, che := metrics.cachingCount.GetMetricWith(prometheus.Labels{
			"operation_name": "GetUsersMetric",
			"status":         string(CachingStatusHit),
		})

		s.Require().NoError(che)
		s.Require().NoError(ch.Write(&metricOut))
		s.Require().Equal(float64(i-1), *metricOut.Counter.Value, "unexpected cache hits metrics")

		cp, cpe := metrics.cachingCount.GetMetricWith(prometheus.Labels{
			"operation_name": "GetBooksMetric",
			"status":         string(CachingStatusPass),
		})

		s.Require().NoError(cpe)
		s.Require().NoError(cp.Write(&metricOut), "can not write metric out")
		s.Require().Equal(float64(i), *metricOut.Counter.Value, "unexpected cache passes metrics")

		oi, oie := metrics.operationInFlight.GetMetricWith(prometheus.Labels{
			"operation_name": "GetUsersMetric",
			"operation_type": "query",
		})

		s.Require().NoError(oie)
		s.Require().NoError(oi.Write(&metricOut), "can not write metric out")
		s.Require().Equal(float64(0), *metricOut.Gauge.Value, "unexpected operation in flight metrics")
	}
}

func (s *HandlerIntegrationTestSuite) TestAdminPurgeQueryResult() {
	testCases := []struct {
		name       string
		mutationOp string
	}{
		{
			name:       "purge_all",
			mutationOp: `{"query": "mutation { result: purgeAll }"}`,
		},
		{
			name:       "purge_by_type",
			mutationOp: `{"query": "mutation { result: purgeType(type: \"UserTest\") }"}`,
		},
		{
			name:       "purge_by_type_key",
			mutationOp: `{"query": "mutation { result: purgeTypeKey(type: \"UserTest\", field: \"id\", key: 1) }"}`,
		},
		{
			name:       "purge_by_operation_name",
			mutationOp: `{"query": "mutation { result: purgeOperation(name: \"GetUsers\") }"}`,
		},
		{
			name:       "purge_by_query_root_field",
			mutationOp: `{"query": "mutation { result: purgeQueryRootField(field: \"users\") }"}`,
		},
	}
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
caching {
	rules {
		default {
			max_age 1h
		}
	}
}
`), "caddyfile")

	booksQueryFunc := func(expectedStatus CachingStatus) {
		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query":"query GetBooks { books { title id } }"}`),
		)
		r.Header.Add("content-type", "application/json")
		resp := tester.AssertResponseCode(r, http.StatusOK)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		expectedStatusString := string(expectedStatus)
		s.Require().Equal(string(body), `{"data":{"books":[{"title":"A - Book 1","id":1},{"title":"A - Book 2","id":2},{"title":"B - Book 1","id":3},{"title":"C - Book 1","id":4}]}}`, "unexpected books response")
		s.Require().Equalf(expectedStatusString, resp.Header.Get("x-cache"), "expected books query should be %s", expectedStatusString)
	}

	booksQueryFunc(CachingStatusMiss)

	for _, testCase := range testCases {
		for i := 0; i <= 3; i++ {
			r, _ := http.NewRequest(
				"POST",
				"http://localhost:9090/graphql",
				strings.NewReader(`{"query":"query GetUsers { users { id name } }"}`),
			)
			r.Header.Add("content-type", "application/json")
			resp := tester.AssertResponseCode(r, http.StatusOK)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			s.Require().Equalf(string(body), `{"data":{"users":[{"id":1,"name":"A"},{"id":2,"name":"B"},{"id":3,"name":"C"}]}}`, "case %s: unexpected response", testCase.name)

			if i == 0 {
				// always miss on first time.
				s.Require().Equalf(string(CachingStatusMiss), resp.Header.Get("x-cache"), "case %s: cache status must MISS on first time", testCase.name)
			} else {
				s.Require().Equalf(string(CachingStatusHit), resp.Header.Get("x-cache"), "case %s: cache status must HIT on next time", testCase.name)
				s.Require().Equalf(strconv.Itoa(i), resp.Header.Get("x-cache-hits"), "case %s: hit times not equal", testCase.name)
			}
		}

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/admin/graphql",
			strings.NewReader(testCase.mutationOp),
		)
		r.Header.Add("content-type", "application/json")
		resp := tester.AssertResponseCode(r, http.StatusOK)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		s.Require().Equal(string(body), `{"data":{"result":true}}`, "case %s: unexpected purge result", testCase.name)

		if testCase.name != "purge_all" {
			booksQueryFunc(CachingStatusHit) // purge users result should not affect books result.
		} else {
			booksQueryFunc(CachingStatusMiss) // purge all should affect books result too.
		}
	}
}

func (s *HandlerIntegrationTestSuite) TestCachingControlRequestHeader() {
	testCases := []struct {
		name                  string
		cc                    string
		expectedCachingStatus CachingStatus
		executeAfter          time.Duration
	}{
		{
			name:                  "first_time_cc_no_store",
			cc:                    "no-store",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "next_time_cc_no_store",
			cc:                    "no-store",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "invalid_cc_first_time_cache_will_miss",
			cc:                    "",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "invalid_cc_next_time_cache_will_hit",
			cc:                    "",
			expectedCachingStatus: CachingStatusHit,
		},
		{
			name:                  "max_age_with_valid_max_stale_cc_result_stale_still_hit",
			executeAfter:          time.Millisecond * 51, // wait for staling
			cc:                    "max-age=1, max-stale=2",
			expectedCachingStatus: CachingStatusHit,
		},
		{
			name:                  "max_age_with_empty_max_stale_cc_result_stale_still_hit",
			executeAfter:          time.Millisecond * 51,
			cc:                    "max-age=1, max-stale",
			expectedCachingStatus: CachingStatusHit,
		},
		{
			name:                  "max_age_with_invalid_max_stale_cc_result_stale_will_miss",
			executeAfter:          time.Millisecond * 2051,
			cc:                    "max-age=1, max-stale=1",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "max_age_without_max_stale_cc_result_stale_will_miss",
			executeAfter:          time.Millisecond * 51,
			cc:                    "max-age=1",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "invalid_min_fresh_cc_will_miss",
			executeAfter:          time.Millisecond * 1001,
			cc:                    "min-fresh=1",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "invalid_max_stale_cc_will_miss",
			executeAfter:          time.Millisecond * 1051,
			cc:                    "max-stale=1",
			expectedCachingStatus: CachingStatusMiss,
		},
		{
			name:                  "empty_max_stale_cc_will_hit",
			executeAfter:          time.Millisecond * 55,
			cc:                    "max-stale",
			expectedCachingStatus: CachingStatusHit,
		},
		{
			name:                  "valid_max_stale_cc_will_hit",
			executeAfter:          time.Millisecond * 55,
			cc:                    "max-stale=1",
			expectedCachingStatus: CachingStatusHit,
		},
	}
	tester := caddytest.NewTester(s.T())
	tester.InitServer(pureCaddyfile, "caddyfile")
	tester.InitServer(fmt.Sprintf(caddyfilePattern, `
caching {
	rules {
		default {
			max_age 50ms
			swr 5s
		}
	}
}
`), "caddyfile")

	for _, testCase := range testCases {
		<-time.After(testCase.executeAfter)

		r, _ := http.NewRequest(
			"POST",
			"http://localhost:9090/graphql",
			strings.NewReader(`{"query":"query GetUsers { users { name } }"}`),
		)
		r.Header.Set("content-type", "application/json")
		r.Header.Set("cache-control", testCase.cc)
		resp := tester.AssertResponseCode(r, http.StatusOK)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		actualCachingStatus := resp.Header.Get("x-cache")

		s.Require().Equalf(string(body), `{"data":{"users":[{"name":"A"},{"name":"B"},{"name":"C"}]}}`, "case %s: unexpected response body", testCase.name)
		s.Require().Equalf(string(testCase.expectedCachingStatus), actualCachingStatus, "case %s: unexpected caching status", testCase.name)
	}
}

func TestHandlerIntegration(t *testing.T) {
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

	suite.Run(t, new(HandlerIntegrationTestSuite))
}
