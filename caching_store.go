package gbox

import (
	"errors"
	"fmt"
	"github.com/coocood/freecache"
	"github.com/eko/gocache/v2/marshaler"
	"github.com/eko/gocache/v2/store"
	"github.com/go-redis/redis/v8"
	"net/url"
	"strconv"
	"sync"
)

var (
	cachingStoreFactories   = make(map[string]CachingStoreFactory)
	cachingStoreFactoriesMu sync.RWMutex
)

func init() {
	RegisterCachingStoreFactory("redis", RedisCachingStoreFactory)
	RegisterCachingStoreFactory("freecache", FreeCacheStoreFactory)
}

type CachingStore struct {
	*marshaler.Marshaler
	close func() error
}

type CachingStoreFactory = func(u *url.URL) (*CachingStore, error)

func RegisterCachingStoreFactory(schema string, factory CachingStoreFactory) {
	cachingStoreFactoriesMu.Lock()
	defer cachingStoreFactoriesMu.Unlock()

	cachingStoreFactories[schema] = factory
}

func NewCachingStore(u *url.URL) (*CachingStore, error) {
	cachingStoreFactoriesMu.RLock()
	defer cachingStoreFactoriesMu.RUnlock()

	if factory, ok := cachingStoreFactories[u.Scheme]; !ok {
		return nil, fmt.Errorf("caching store schema: %s is not support", u.Scheme)
	} else {
		return factory(u)
	}
}

func FreeCacheStoreFactory(u *url.URL) (*CachingStore, error) {
	q := u.Query()
	cacheSize := q.Get("cache_size")

	if cacheSize == "" {
		return nil, errors.New("cache_size must be set explicit")
	}

	cacheSizeInt, err := strconv.Atoi(cacheSize)

	if err != nil {
		return nil, fmt.Errorf("`cache_size` param should be numeric string, %s given", cacheSize)
	}

	client := freecache.NewCache(cacheSizeInt)
	freeCacheStore := store.NewFreecache(client, nil)

	return &CachingStore{
		Marshaler: marshaler.New(freeCacheStore),
		close: func() error {
			client.Clear()

			return nil
		},
	}, nil
}

func RedisCachingStoreFactory(u *url.URL) (*CachingStore, error) {
	q := u.Query()
	opts := &redis.Options{
		Addr: u.Host,
	}

	if v := q.Get("db"); v != "" {
		db, err := strconv.Atoi(v)

		if err != nil {
			return nil, fmt.Errorf("`db` param should be numeric string, %s given", v)
		}

		opts.DB = db
	}

	user := u.User.Username()
	password, hasPassword := u.User.Password()

	if !hasPassword {
		opts.Password = user
	} else {
		opts.Username = user
		opts.Password = password
	}

	client := redis.NewClient(opts)
	redisStore := store.NewRedis(client, nil)

	return &CachingStore{
		Marshaler: marshaler.New(redisStore),
		close: func() error {
			return client.Close()
		},
	}, nil
}
