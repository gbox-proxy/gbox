package gbox

import (
	"encoding/json"

	"github.com/jensneuse/graphql-go-tools/pkg/pool"
)

// CachingVary using to compute query result cache key by http request cookies and headers.
type CachingVary struct {
	// Headers names for identifier query result cache key.
	Headers []string `json:"headers,omitempty"`

	// Cookies names for identifier query result cache key.
	Cookies []string `json:"cookies,omitempty"`
}

type CachingVaries map[string]*CachingVary

func (varies CachingVaries) hash() (uint64, error) {
	if varies == nil {
		return 0, nil
	}

	hash := pool.Hash64.Get()
	hash.Reset()
	defer pool.Hash64.Put(hash)

	if err := json.NewEncoder(hash).Encode(varies); err != nil {
		return 0, err
	}

	return hash.Sum64(), nil
}
