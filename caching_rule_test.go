package gbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCachingRulesHash(t *testing.T) {
	var rules CachingRules
	hash, err := rules.hash()

	require.NoError(t, err)
	require.Equal(t, uint64(0), hash)

	rules = CachingRules{
		"default": &CachingRule{
			MaxAge: 1,
			Swr:    1,
		},
	}

	hash, err = rules.hash()

	require.NoError(t, err)
	require.Greater(t, hash, uint64(0))
}
