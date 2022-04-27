package gbox

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/eko/gocache/v2/store"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCaching_PurgeQueryResultByOperationName(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store:  s,
		logger: zap.NewNop(),
	}
	v := &struct{}{}

	_, err := c.store.Get(context.Background(), "test", v)
	require.Error(t, err)

	c.store.Set(context.Background(), "test", v, &store.Options{
		Tags: []string{fmt.Sprintf(cachingTagOperationPattern, "test")},
	})

	_, err = c.store.Get(context.Background(), "test", v)

	require.NoError(t, err)
	require.NoError(t, c.PurgeQueryResultByOperationName(context.Background(), "test"))

	_, err = c.store.Get(context.Background(), "test", v)
	require.Error(t, err)
}

func TestCaching_PurgeQueryResultBySchema(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store:  s,
		logger: zap.NewNop(),
	}
	schema, _ := graphql.NewSchemaFromString(`
type Query {
	test: String!
}
`)
	schema.Normalize()
	schemaHash, _ := schema.Hash()
	v := &struct{}{}

	_, err := c.store.Get(context.Background(), "test", v)
	require.Error(t, err)

	c.store.Set(context.Background(), "test", v, &store.Options{
		Tags: []string{fmt.Sprintf(cachingTagSchemaHashPattern, schemaHash)},
	})

	_, err = c.store.Get(context.Background(), "test", v)

	require.NoError(t, err)
	require.NoError(t, c.PurgeQueryResultBySchema(context.Background(), schema))

	_, err = c.store.Get(context.Background(), "test", v)
	require.Error(t, err)
}

func TestCaching_PurgeQueryResultByTypeKey(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store:  s,
		logger: zap.NewNop(),
	}
	v := &struct{}{}

	_, err := c.store.Get(context.Background(), "test", v)
	require.Error(t, err)

	c.store.Set(context.Background(), "test", v, &store.Options{
		Tags: []string{fmt.Sprintf(cachingTagTypeKeyPattern, "a", "b", "c")},
	})

	_, err = c.store.Get(context.Background(), "test", v)

	require.NoError(t, err)
	require.NoError(t, c.PurgeQueryResultByTypeKey(context.Background(), "a", "b", "c"))

	_, err = c.store.Get(context.Background(), "test", v)
	require.Error(t, err)
}

func TestCaching_PurgeQueryResultByTypeField(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store:  s,
		logger: zap.NewNop(),
	}
	v := &struct{}{}

	_, err := c.store.Get(context.Background(), "test", v)
	require.Error(t, err)

	c.store.Set(context.Background(), "test", v, &store.Options{
		Tags: []string{fmt.Sprintf(cachingTagTypeFieldPattern, "a", "b")},
	})

	_, err = c.store.Get(context.Background(), "test", v)

	require.NoError(t, err)
	require.NoError(t, c.PurgeQueryResultByTypeField(context.Background(), "a", "b"))

	_, err = c.store.Get(context.Background(), "test", v)
	require.Error(t, err)
}

func TestCaching_PurgeQueryResultByTypeName(t *testing.T) {
	u, _ := url.Parse("freecache://?cache_size=1000000")
	s, _ := NewCachingStore(u)
	c := &Caching{
		store:  s,
		logger: zap.NewNop(),
	}
	v := &struct{}{}

	_, err := c.store.Get(context.Background(), "test", v)
	require.Error(t, err)

	c.store.Set(context.Background(), "test", v, &store.Options{
		Tags: []string{fmt.Sprintf(cachingTagTypePattern, "a")},
	})

	_, err = c.store.Get(context.Background(), "test", v)

	require.NoError(t, err)
	require.NoError(t, c.PurgeQueryResultByTypeName(context.Background(), "a"))

	_, err = c.store.Get(context.Background(), "test", v)
	require.Error(t, err)
}
