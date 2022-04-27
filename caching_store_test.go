package gbox

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCachingStore(t *testing.T) {
	testCases := map[string]struct {
		url              string
		expectedErrorMsg string
	}{
		"redis": {
			url: "redis://redis",
		},
		"freecache": {
			url: "freecache://?cache_size=1024",
		},
		"unknown": {
			url:              "unknown://unknown",
			expectedErrorMsg: "caching store schema: unknown is not support",
		},
	}

	for name, testCase := range testCases {
		u, _ := url.Parse(testCase.url)
		_, err := NewCachingStore(u)

		if testCase.expectedErrorMsg == "" {
			require.NoErrorf(t, err, "case %s: unexpected error", name)
		} else {
			require.Errorf(t, err, "case %s: should be error", name)
			require.Equalf(t, testCase.expectedErrorMsg, err.Error(), "case %s: unexpected error message", name)
		}
	}
}

func TestFreeCacheStoreFactory(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1024")
	_, e := FreeCacheStoreFactory(u)

	require.NoError(t, e)

	u, _ = url.Parse("freecache://") // missing cache size

	_, e = NewCachingStore(u)

	require.Error(t, e)
	require.Equal(t, "cache_size must be set explicit", e.Error())
}

func TestRedisCachingStoreFactory(t *testing.T) {
	u, _ := url.Parse("redis://redis")
	_, e := RedisCachingStoreFactory(u)

	require.NoError(t, e)

	u, _ = url.Parse("redis://redis?db=xyz")
	_, e = RedisCachingStoreFactory(u)

	require.Error(t, e)
	require.Equal(t, "`db` param should be numeric string, xyz given", e.Error())
}
