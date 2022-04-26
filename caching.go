package gbox

import (
	"context"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"go.uber.org/zap"
	"net/url"
)

var (
	cachingStores = caddy.NewUsagePool()
)

type (
	CachingStatus string
)

const (
	CachingStatusPass CachingStatus = "PASS"
	CachingStatusHit  CachingStatus = "HIT"
	CachingStatusMiss CachingStatus = "MISS"
)

type Caching struct {
	// Storage DSN currently support redis and freecache only.
	// Redis example:
	// redis://username:password@localhost:6379?db=0&max_retries=3
	// more dsn options see at https://github.com/go-redis/redis/blob/v8.11.5/options.go#L31
	// Freecache example:
	// freecache://?cache_size=104857600
	// If not set it will be freecache://?cache_size=104857600 (cache size 100MB)
	StoreDsn string `json:"store_dsn,omitempty"`

	// Caching rules
	Rules CachingRules `json:"rules,omitempty"`

	// Caching varies
	Varies CachingVaries `json:"varies,omitempty"`

	// GraphQL type fields will be used to detect change of cached query results when user execute mutation query.
	// Example when execute mutation query bellow:
	// mutation { updateUser { id } }
	// if `updateUser` field have type User and id returning in example above is 1, all cache results of user id 1 will be purged.
	// If not set default value of it will be `id` for all types.
	TypeKeys graphql.RequestTypes `json:"type_keys,omitempty"`

	// Auto invalidate query result cached by mutation result type keys
	// Example: if you had cached query result of User type, when you make mutation query and result
	// of this query have type User with id's 3, all cached query result related with id 3 of User type will be purged.
	AutoInvalidate bool

	// Add debug headers like query result cache key,
	// plan cache key and query result had types keys or not...
	DebugHeaders bool

	logger              *zap.Logger
	store               *CachingStore
	ctxBackground       context.Context
	ctxBackgroundCancel func()
	cachingMetrics
}

type cachingStoreDestructor struct {
	store *CachingStore
}

func (c *cachingStoreDestructor) Destruct() error {
	return c.store.close()
}

func (c *Caching) withLogger(l *zap.Logger) {
	c.logger = l
}

func (c *Caching) withMetrics(m cachingMetrics) {
	c.cachingMetrics = m
}

func (c *Caching) Provision(ctx caddy.Context) error {
	repl := caddy.NewReplacer()
	c.StoreDsn = repl.ReplaceKnown(c.StoreDsn, "")
	c.ctxBackground, c.ctxBackgroundCancel = context.WithCancel(context.Background())

	if c.StoreDsn == "" {
		c.StoreDsn = "freecache://?cache_size=104857600"
	}

	destructor, _, err := cachingStores.LoadOrNew(c.StoreDsn, func() (caddy.Destructor, error) {
		var u *url.URL
		var err error
		var store *CachingStore
		u, err = url.Parse(c.StoreDsn)

		if err != nil {
			return nil, err
		}

		store, err = NewCachingStore(u)

		if err != nil {
			return nil, err
		}

		return &cachingStoreDestructor{
			store: store,
		}, nil
	})

	if err != nil {
		return err
	}

	c.store = destructor.(*cachingStoreDestructor).store

	return nil
}

func (c *Caching) Validate() error {
	for ruleName, rule := range c.Rules {
		for vary := range rule.Varies {
			if _, ok := c.Varies[vary]; !ok {
				return fmt.Errorf("caching rule %s, configured vary: %s does not exist", ruleName, vary)
			}
		}

		if rule.MaxAge <= 0 {
			return fmt.Errorf("caching rule %s, max age must greater than zero", ruleName)
		}
	}

	return nil
}

func (c *Caching) Cleanup() error {
	c.ctxBackgroundCancel()
	_, err := cachingStores.Delete(c.StoreDsn)

	return err
}

// Interface guards
var (
	_ caddy.Provisioner  = (*Caching)(nil)
	_ caddy.Validator    = (*Caching)(nil)
	_ caddy.CleanerUpper = (*Caching)(nil)
)
