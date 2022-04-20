package gbox

import (
	"context"
	"encoding/json"
	"github.com/caddyserver/caddy/v2"
	"github.com/eko/gocache/v2/store"
	"github.com/pquerna/cachecontrol/cacheobject"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	CachingQueryResultStale cachingQueryResultStatus = "STALE"
	CachingQueryResultValid                          = "VALID"
)

type cachingQueryResultStatus string

type cachingQueryResult struct {
	Header     http.Header
	Body       json.RawMessage
	HitTime    uint64
	CreatedAt  time.Time
	Expiration time.Duration
	MaxAge     *caddy.Duration
	Swr        *caddy.Duration
	Tags       []string

	plan *cachingPlan
}

func (c *Caching) getCachingQueryResult(ctx context.Context, plan *cachingPlan) (*cachingQueryResult, error) {
	result := &cachingQueryResult{
		plan: plan,
	}

	if _, err := c.store.Get(ctx, plan.queryResultCacheKey, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Caching) cachingQueryResult(request *cachingRequest, plan *cachingPlan, body []byte, header http.Header) error {
	tags := make(cachingTags)
	tagAnalyzer := newCachingTagAnalyzer(request, c.TypeKeys)

	if err := tagAnalyzer.AnalyzeResult(body, plan.Types, tags); err != nil {
		return err
	}

	result := &cachingQueryResult{
		Body:      body,
		Header:    header,
		CreatedAt: time.Now(),
		MaxAge:    plan.MaxAge,
		Swr:       plan.Swr,
		Tags:      tags.ToSlice(),
	}

	result.normalizeHeader()
	result.computeExpiration()

	return c.store.Set(c.ctxBackground, plan.queryResultCacheKey, result, result.storeOpts())
}

func (c *Caching) increaseQueryResultHitTimes(ctx context.Context, r *cachingQueryResult) {
	r.HitTime += 1
	err := c.store.Set(ctx, r.plan.queryResultCacheKey, r, r.storeOpts())

	if err != nil {
		c.logger.Error("increase query result hit times failed", zap.String("cache_key", r.plan.queryResultCacheKey), zap.Error(err))
	}
}

func (r *cachingQueryResult) Status() cachingQueryResultStatus {
	if r.Expiration == 0 || r.MaxAge == nil || time.Duration(*r.MaxAge) >= r.Age() {
		return CachingQueryResultValid
	}

	return CachingQueryResultStale
}

func (r *cachingQueryResult) storeOpts() *store.Options {
	return &store.Options{
		Tags:       r.Tags,
		Expiration: r.Expiration,
	}
}

func (r *cachingQueryResult) computeExpiration() {
	var expiration time.Duration

	if r.MaxAge != nil {
		expiration += time.Duration(*r.MaxAge)
	}

	if r.Swr != nil {
		expiration += time.Duration(*r.Swr)
	}

	r.Expiration = expiration
}

// ValidFor check caching result still valid with cache control cachingRequest directives
// https://datatracker.ietf.org/doc/html/rfc7234#section-5.2.1
func (r *cachingQueryResult) ValidFor(cc *cacheobject.RequestCacheDirectives) bool {
	status := r.Status()
	age := r.Age()

	if cc.NoCache && status == CachingQueryResultStale {
		return false
	}

	if cc.MinFresh != -1 {
		var maxAge time.Duration
		d := age + time.Duration(cc.MinFresh)*time.Second

		if r.MaxAge != nil {
			maxAge = time.Duration(*r.MaxAge)
		}

		if d > maxAge {
			return false
		}
	}

	// max-age request
	if cc.MaxAge != -1 {
		d := time.Duration(cc.MaxAge) * time.Second

		if d >= age && status == CachingQueryResultValid {
			return true
		}

		// max-age with max-stale
		if cc.MaxStaleSet && status == CachingQueryResultStale {
			// client is willing to accept a stale response of any age.
			if cc.MaxStale == -1 {
				return true
			}

			d += time.Duration(cc.MaxStale) * time.Second

			return d >= age
		}

		return false
	}

	// max-stale only
	if cc.MaxStaleSet {
		if cc.MaxStale == -1 || status == CachingQueryResultValid {
			return true
		}

		d := time.Duration(*r.MaxAge) + time.Duration(cc.MaxStale)*time.Second

		return d >= age
	}

	return true
}

func (r *cachingQueryResult) Age() time.Duration {
	return time.Now().Sub(r.CreatedAt)
}

func (r *cachingQueryResult) normalizeHeader() {
	r.Header.Del("date")
	r.Header.Del("server")
}
