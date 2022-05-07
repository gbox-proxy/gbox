package gbox

import (
	"encoding/json"

	"github.com/caddyserver/caddy/v2"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/pool"
)

type CachingRule struct {
	// GraphQL type to cache
	// ex: `User` will cache all query results have type User
	// ex: `User { is_admin }` will cache all query results have type User and have field `is_admin`.
	// If not set this rule will match all types.
	Types graphql.RequestTypes `json:"types,omitempty"`

	// how long query results that match the rule types should be store.
	MaxAge caddy.Duration `json:"max_age,omitempty"`

	// how long stale query results that match the rule types should be served while fresh data is already being fetched in the background.
	Swr caddy.Duration `json:"swr,omitempty"`

	// Varies name apply to query results that match the rule types.
	// If not set query results will cache public.
	Varies []string `json:"varies,omitempty"`
}

type CachingRules map[string]*CachingRule

func (rules CachingRules) hash() (uint64, error) {
	if rules == nil {
		return 0, nil
	}

	hash := pool.Hash64.Get()
	hash.Reset()
	defer pool.Hash64.Put(hash)

	if err := json.NewEncoder(hash).Encode(rules); err != nil {
		return 0, err
	}

	return hash.Sum64(), nil
}
