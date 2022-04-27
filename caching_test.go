package gbox

import (
	"context"
	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
)

func TestCaching_Cleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Caching{
		ctxBackground:       ctx,
		ctxBackgroundCancel: cancel,
		StoreDsn:            "test",
		logger:              zap.NewNop(),
	}

	_, loaded := cachingStores.LoadOrStore(c.StoreDsn, "b")
	require.False(t, loaded)

	require.NoError(t, ctx.Err())
	require.NoError(t, c.Cleanup())
	require.Error(t, ctx.Err())

	_, loaded = cachingStores.LoadOrStore(c.StoreDsn, "b")
	require.False(t, loaded)
}

func TestCaching_Validate(t *testing.T) {
	testCases := map[string]struct {
		caching          *Caching
		expectedErrorMsg string
	}{
		"valid_rules_without_varies": {
			caching: &Caching{
				Rules: CachingRules{
					"default": &CachingRule{
						MaxAge: 1,
					},
				},
			},
		},
		"valid_rules_with_varies": {
			caching: &Caching{
				Varies: map[string]*CachingVary{
					"test": &CachingVary{},
				},
				Rules: CachingRules{
					"default": &CachingRule{
						MaxAge: 1,
						Varies: map[string]struct{}{
							"test": struct{}{},
						},
					},
				},
			},
		},
		"invalid_rules_max_age": {
			expectedErrorMsg: "caching rule default, max age must greater than zero",
			caching: &Caching{
				Rules: CachingRules{
					"default": &CachingRule{},
				},
			},
		},
		"rules_vary_name_not_exist": {
			expectedErrorMsg: "caching rule default, configured vary: test does not exist",
			caching: &Caching{
				Rules: CachingRules{
					"default": &CachingRule{
						MaxAge: 1,
						Varies: map[string]struct{}{
							"test": struct{}{},
						},
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		err := testCase.caching.Validate()

		if testCase.expectedErrorMsg != "" {
			require.Errorf(t, err, "case %s: expected error but not", name)
			require.Equalf(t, testCase.expectedErrorMsg, err.Error(), "case %s: unexpected error message", name)
		} else {
			require.NoErrorf(t, err, "case %s: should not error", name)
		}
	}
}

func TestCaching_Provision(t *testing.T) {
	c := &Caching{
		StoreDsn: "redis://test",
	}

	require.NoError(t, c.Provision(caddy.Context{}))
	require.NotNil(t, c.store)
	require.NotNil(t, c.ctxBackground)
	require.NotNil(t, c.ctxBackgroundCancel)
}
