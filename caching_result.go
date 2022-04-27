package gbox

import (
	"context"
	"encoding/json"
	"github.com/caddyserver/caddy/v2"
	"github.com/eko/gocache/v2/store"
	"github.com/pquerna/cachecontrol/cacheobject"
	"net/http"
	"time"
)

const (
	CachingQueryResultStale cachingQueryResultStatus = "STALE"
	CachingQueryResultValid cachingQueryResultStatus = "VALID"
)

type cachingQueryResultStatus string

type cachingQueryResult struct {
	Header     http.Header
	Body       json.RawMessage
	HitTime    uint64
	CreatedAt  time.Time
	Expiration time.Duration
	MaxAge     caddy.Duration
	Swr        caddy.Duration
	Tags       cachingTags

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

func (c *Caching) cachingQueryResult(ctx context.Context, request *cachingRequest, plan *cachingPlan, body []byte, header http.Header) (err error) {
	tags := make(cachingTags)
	tagAnalyzer := newCachingTagAnalyzer(request, c.TypeKeys)

	if err = tagAnalyzer.AnalyzeResult(body, plan.Types, tags); err != nil {
		return err
	}

	result := &cachingQueryResult{
		Body:       body,
		Header:     header,
		CreatedAt:  time.Now(),
		MaxAge:     plan.MaxAge,
		Swr:        plan.Swr,
		Tags:       tags,
		Expiration: time.Duration(plan.MaxAge) + time.Duration(plan.Swr),
	}

	result.normalizeHeader()

	return c.store.Set(ctx, plan.queryResultCacheKey, result, &store.Options{
		Tags:       tags.ToSlice(),
		Expiration: result.Expiration,
	})
}

func (c *Caching) increaseQueryResultHitTimes(ctx context.Context, r *cachingQueryResult) error {
	r.HitTime++

	return c.store.Set(ctx, r.plan.queryResultCacheKey, r, &store.Options{
		Expiration: r.Expiration - time.Since(r.CreatedAt),
	})
}

func (r *cachingQueryResult) Status() cachingQueryResultStatus {
	if time.Duration(r.MaxAge) >= r.Age() {
		return CachingQueryResultValid
	}

	return CachingQueryResultStale
}

// ValidFor check caching result still valid with cache control directives
// https://datatracker.ietf.org/doc/html/rfc7234#section-5.2.1
func (r *cachingQueryResult) ValidFor(cc *cacheobject.RequestCacheDirectives) bool {
	status := r.Status()
	age := r.Age()

	if cc.NoCache && status == CachingQueryResultStale {
		return false
	}

	if cc.MinFresh != -1 {
		maxAge := time.Duration(r.MaxAge)
		d := age + time.Duration(cc.MinFresh)*time.Second

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
		if (cc.MaxStaleSet || cc.MaxStale != -1) && status == CachingQueryResultStale {
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
	if cc.MaxStaleSet || cc.MaxStale != -1 {
		if cc.MaxStale == -1 || status == CachingQueryResultValid {
			return true
		}

		d := time.Duration(r.MaxAge) + time.Duration(cc.MaxStale)*time.Second

		return d >= age
	}

	return true
}

func (r *cachingQueryResult) Age() time.Duration {
	return time.Since(r.CreatedAt)
}

func (r *cachingQueryResult) normalizeHeader() {
	r.Header.Del("date")
	r.Header.Del("server")
}
