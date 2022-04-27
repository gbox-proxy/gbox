package gbox

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/require"
)

func TestCaddyfile(t *testing.T) {
	testCases := map[string]struct {
		name                         string
		disabledIntrospection        string
		disabledPlaygrounds          string
		enabledCaching               string
		enabledComplexity            string
		enabledCachingAutoInvalidate string
	}{
		"enabled_all_features": {
			enabledCaching:               "true",
			enabledCachingAutoInvalidate: "true",
			enabledComplexity:            "true",
			disabledIntrospection:        "false",
			disabledPlaygrounds:          "false",
		},
		"disabled_all_features": {
			enabledCaching:               "false",
			enabledCachingAutoInvalidate: "false",
			enabledComplexity:            "false",
			disabledIntrospection:        "true",
			disabledPlaygrounds:          "true",
		},
		"enabled_caching_and_disabled_caching_auto_invalidate": {
			enabledCaching:               "true",
			enabledCachingAutoInvalidate: "false",
			enabledComplexity:            "false",
			disabledIntrospection:        "true",
			disabledPlaygrounds:          "true",
		},
	}

	for name, testCase := range testCases {
		h := &Handler{}
		d := caddyfile.NewTestDispenser(fmt.Sprintf(`
gbox {
	upstream http://localhost:9091
	complexity {
		enabled %s
		max_depth 3
		max_complexity 2
		node_count_limit 1
	}
	disabled_playgrounds %s
	disabled_introspection %s
	caching {
		enabled %s
		auto_invalidate_cache %s
		varies {
			authorization {
				headers Authorization
				cookies session_id
			}
		}
		rules {
			rule1 {
				max_age 10m
			}
			rule2 {
				max_age 5m
			}
		}
	}
}
`, testCase.enabledComplexity, testCase.disabledPlaygrounds, testCase.disabledIntrospection, testCase.enabledCaching, testCase.enabledCachingAutoInvalidate))
		require.NoErrorf(t, h.UnmarshalCaddyfile(d), "case %s: unmarshal caddy file error", name)
		require.Equalf(t, h.Upstream, "http://localhost:9091", "case %s: invalid upstream", name)
		require.NotNilf(t, h.RewriteRaw, "case %s: rewrite raw should be set", name)
		require.NotNilf(t, h.ReverseProxyRaw, "case %s: reverse proxy raw should be set", name)

		enabledComplexity, _ := strconv.ParseBool(testCase.enabledComplexity)
		enabledCaching, _ := strconv.ParseBool(testCase.enabledCaching)
		disabledPlaygrounds, _ := strconv.ParseBool(testCase.disabledPlaygrounds)
		disabledIntrospection, _ := strconv.ParseBool(testCase.disabledIntrospection)
		enabledCachingAutoInvalidate, _ := strconv.ParseBool(testCase.enabledCachingAutoInvalidate)

		if enabledCaching {
			rule1, rule1Exist := h.Caching.Rules["rule1"]
			rule2, rule2Exist := h.Caching.Rules["rule2"]

			require.Equalf(t, enabledCachingAutoInvalidate, h.Caching.AutoInvalidate, "case %s: unexpected caching auto invalidate", name)
			require.Truef(t, rule1Exist, "case %s: rule1 should be exist", name)
			require.Truef(t, rule2Exist, "case %s: rule2 should be exist", name)
			require.Equal(t, caddy.Duration(time.Minute*10), rule1.MaxAge, "case %s: unexpected rule1 max age", name)
			require.Equal(t, caddy.Duration(time.Minute*5), rule2.MaxAge, "case %s: unexpected rule2 max age", name)
		} else {
			require.Nilf(t, h.Caching, "case %s: caching should be nil if not enabled", name)
		}

		if enabledComplexity {
			require.Equalf(t, 3, h.Complexity.MaxDepth, "case %s: max depth should be 3", name)
			require.Equalf(t, 2, h.Complexity.MaxComplexity, "case %s: max complexity should be 2", name)
			require.Equalf(t, 1, h.Complexity.NodeCountLimit, "case %s: node count limit should be 1", name)
		} else {
			require.Nilf(t, h.Complexity, "case %s: complexity should be nil if not enabled", name)
		}

		require.Equalf(t, disabledIntrospection, h.DisabledIntrospection, "case %s: unexpected disabled introspection", name)
		require.Equalf(t, disabledPlaygrounds, h.DisabledPlaygrounds, "case %s: unexpected disabled playgrounds", name)
	}
}

func TestCaddyfileErrors(t *testing.T) {
	testCases := map[string]struct {
		config   string
		errorMsg string
	}{
		"unexpected_gbox_subdirective": {
			config:   `unknown`,
			errorMsg: `unrecognized subdirective unknown`,
		},
		"blank_gbox_disabled_introspection": {
			config: `
disabled_introspection
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_disabled_introspection": {
			config: `
disabled_introspection invalid
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_complexity_enabled": {
			config: `
complexity {
	enabled
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_complexity_enabled": {
			config: `
complexity {
	enabled invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_complexity_max_complexity": {
			config: `
complexity {
	max_complexity
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_complexity_max_complexity": {
			config: `
complexity {
	max_complexity invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_complexity_max_depth": {
			config: `
complexity {
	max_depth
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_complexity_max_depth": {
			config: `
complexity {
	max_depth invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_complexity_node_count_limit": {
			config: `
complexity {
	node_count_limit
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_complexity_node_count_limit": {
			config: `
complexity {
	max_depth invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"unexpected_gbox_complexity_subdirective": {
			config: `
complexity {
	unknown
}
`,
			errorMsg: `unrecognized subdirective unknown`,
		},
		"unexpected_gbox_caching_subdirective": {
			config: `
caching {
	unknown
}
`,
			errorMsg: `unrecognized subdirective unknown`,
		},
		"blank_gbox_caching_enabled": {
			config: `
caching {
	enabled
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_caching_enabled": {
			config: `
caching {
	enabled invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_caching_auto_invalidate_cache": {
			config: `
caching {
	auto_invalidate_cache
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_caching_auto_invalidate_cache": {
			config: `
caching {
	auto_invalidate_cache invalid
}
`,
			errorMsg: `invalid syntax`,
		},
		"blank_gbox_caching_store_dsn": {
			config: `
caching {
	store_dsn
}
`,
			errorMsg: `Wrong argument count`,
		},
		"invalid_syntax_gbox_caching_store_dsn": {
			config: `
caching {
	store_dsn !://a
}
`,
			errorMsg: `first path segment in URL cannot contain colon`,
		},
		"unexpected_gbox_caching_rules_subdirective": {
			config: `
caching {
	rules {
		a {
			unknown
		}
	}
}
`,
			errorMsg: `unrecognized subdirective unknown`,
		},
		"unexpected_gbox_caching_varies_subdirective": {
			config: `
caching {
	varies {
		a {
			unknown
		}
	}
}
`,
			errorMsg: `unrecognized subdirective unknown`,
		},
		"unexpected_gbox_caching_type_keys": {
			config: `
caching {
	type_keys {
		UserTest
	}
}
`,
			errorMsg: `Wrong argument count`,
		},
	}

	for name, testCase := range testCases {
		h := &Handler{}
		d := caddyfile.NewTestDispenser(fmt.Sprintf(`
gbox {
	%s
}
`, testCase.config))
		e := h.UnmarshalCaddyfile(d)
		require.Errorf(t, e, "case %s: should be invalid", name)
		require.Contains(t, e.Error(), testCase.errorMsg, "case %s: unexpected error message", name)
	}
}
