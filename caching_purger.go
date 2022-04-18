package gbox

import (
	"context"
	"fmt"
	"github.com/eko/gocache/v2/store"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"strconv"
)

func (c *Caching) purgeQueryResultByMutationResult(request *cachingRequest, result []byte) error {
	foundTags := make(cachingTags)
	tagAnalyzer := newCachingTagAnalyzer(request, c.TypeKeys)

	if err := tagAnalyzer.AnalyzeResult(result, nil, foundTags); err != nil {
		return err
	}

	purgeTags := foundTags.TypeKeys().ToSlice()

	return c.purgeQueryResultByTags(c.ctxBackground, purgeTags)
}

func (c *Caching) purgeQueryResultBySchema(ctx context.Context, schema *graphql.Schema) error {
	hash, _ := schema.Hash()
	tag := fmt.Sprintf(cachingTagSchemaHashPattern, hash)

	return c.purgeQueryResultByTags(ctx, []string{tag})
}

func (c *Caching) PurgeQueryResultByOperationName(ctx context.Context, name string) error {
	return c.purgeQueryResultByTags(ctx, []string{fmt.Sprintf(cachingTagOperationPattern, name)})
}

func (c *Caching) PurgeQueryResultByTypeName(ctx context.Context, name string) error {
	return c.purgeQueryResultByTags(ctx, []string{fmt.Sprintf(cachingTagTypePattern, name)})
}

func (c *Caching) PurgeQueryResultByTypeField(ctx context.Context, typeName, fieldName string) error {
	return c.purgeQueryResultByTags(ctx, []string{fmt.Sprintf(cachingTagTypeFieldPattern, typeName, fieldName)})
}

func (c *Caching) PurgeQueryResultByTypeKey(ctx context.Context, typeName, fieldName string, value interface{}) error {
	var cacheKey string

	switch v := value.(type) {
	case string:
		cacheKey = fmt.Sprintf(cachingTagTypeKeyPattern, typeName, fieldName, v)
		return c.purgeQueryResultByTags(ctx, []string{cacheKey})
	case int:
		cacheKey = fmt.Sprintf(cachingTagTypeKeyPattern, typeName, fieldName, strconv.Itoa(v))
		return c.purgeQueryResultByTags(ctx, []string{cacheKey})
	default:
		return fmt.Errorf("only support purging type key value int or string, got %T", v)
	}
}

func (c *Caching) purgeQueryResultByTags(ctx context.Context, tags []string) error {
	var err error

	c.logger.Debug("purging query result by tags", zap.Strings("tags", tags))

	for _, t := range tags {
		// because store invalidate method will be stopped on first error,
		// so we need to invalidate tag by tag.
		if e := c.store.Invalidate(ctx, store.InvalidateOptions{Tags: []string{t}}); e != nil {
			if err == nil {
				err = e
			} else {
				err = errors.WithMessage(err, e.Error())
			}
		}
	}

	return err
}
