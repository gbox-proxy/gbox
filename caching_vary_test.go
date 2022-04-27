package gbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCachingVariesHash(t *testing.T) {
	var varies CachingVaries
	hash, err := varies.hash()

	require.NoError(t, err)
	require.Equal(t, uint64(0), hash)

	varies = CachingVaries{
		"default": &CachingVary{
			Cookies: map[string]struct{}{
				"session": {},
			},
		},
	}

	hash, err = varies.hash()

	require.NoError(t, err)
	require.Greater(t, hash, uint64(0))
}
