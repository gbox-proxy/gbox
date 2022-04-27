package gbox

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"go.uber.org/zap"
)

var ErrHandleUnknownOperationTypeError = errors.New("unknown operation type")

// HandleRequest caching GraphQL query result by configured rules and varies.
func (c *Caching) HandleRequest(w http.ResponseWriter, r *cachingRequest, h caddyhttp.HandlerFunc) error {
	// Remove `accept-encoding` header to prevent response body encoded when forward request to upstream
	// encode directive had read this header, safe to delete it.
	r.httpRequest.Header.Del("accept-encoding")
	operationType, _ := r.gqlRequest.OperationType()

	// nolint:exhaustive
	switch operationType {
	case graphql.OperationTypeQuery:
		return c.handleQueryRequest(w, r, h)
	case graphql.OperationTypeMutation:
		return c.handleMutationRequest(w, r, h)
	}

	return ErrHandleUnknownOperationTypeError
}

func (c *Caching) handleQueryRequest(w http.ResponseWriter, r *cachingRequest, h caddyhttp.HandlerFunc) (err error) {
	var plan *cachingPlan
	report := &operationreport.Report{}
	plan, err = c.getCachingPlan(r)

	if err != nil {
		report.AddInternalError(err)

		return report
	}

	status, result := c.resolvePlan(r, plan)
	defer c.addMetricsCaching(r.gqlRequest, status)

	switch status {
	case CachingStatusMiss:
		bodyBuff := bufferPool.Get().(*bytes.Buffer)
		defer bufferPool.Put(bodyBuff)
		bodyBuff.Reset()

		crw := newCachingResponseWriter(bodyBuff)

		if err = h(crw, r.httpRequest); err != nil {
			return err
		}

		defer func() {
			c.addCachingResponseHeaders(status, result, plan, w.Header())
			err = crw.WriteResponse(w)
		}()

		shouldCache := false
		mt, _, _ := mime.ParseMediaType(crw.header.Get("content-type"))

		if crw.Status() == http.StatusOK && mt == "application/json" {
			// respect no-store directive
			// https://datatracker.ietf.org/doc/html/rfc7234#section-5.2.1.5
			shouldCache = r.cacheControl == nil || !r.cacheControl.NoStore
		}

		if !shouldCache {
			return err
		}

		bodyBuffCopy := bufferPool.Get().(*bytes.Buffer)
		bodyBuffCopy.Reset()
		bodyBuffCopy.Write(crw.buffer.Bytes())

		go func(header http.Header) {
			defer bufferPool.Put(bodyBuffCopy)
			err := c.cachingQueryResult(c.ctxBackground, r, plan, bodyBuffCopy.Bytes(), header)

			if err != nil {
				c.logger.Info("fail to cache query result", zap.Error(err))
			} else {
				c.logger.Info("caching query result successful", zap.String("cache_key", plan.queryResultCacheKey))
			}
		}(crw.Header().Clone())
	case CachingStatusHit:
		for header, values := range result.Header {
			w.Header()[header] = values
		}

		c.addCachingResponseHeaders(status, result, plan, w.Header())
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(result.Body)

		if err != nil || result.Status() != CachingQueryResultStale {
			return err
		}

		r.httpRequest = prepareSwrHTTPRequest(c.ctxBackground, r.httpRequest, w)

		go func() {
			if err := c.swrQueryResult(c.ctxBackground, result, r, h); err != nil {
				c.logger.Info("swr failed, can not update query result", zap.String("cache_key", plan.queryResultCacheKey), zap.Error(err))
			} else {
				c.logger.Info("swr query result successful", zap.String("cache_key", plan.queryResultCacheKey))
			}
		}()
	case CachingStatusPass:
		c.addCachingResponseHeaders(status, result, plan, w.Header())
		err = h(w, r.httpRequest)
	}

	return err
}

func (c *Caching) resolvePlan(r *cachingRequest, p *cachingPlan) (CachingStatus, *cachingQueryResult) {
	if p.Passthrough {
		return CachingStatusPass, nil
	}

	result, _ := c.getCachingQueryResult(r.httpRequest.Context(), p)

	if result != nil && (r.cacheControl == nil || result.ValidFor(r.cacheControl)) {
		err := c.increaseQueryResultHitTimes(r.httpRequest.Context(), result)

		if err != nil {
			c.logger.Error("increase query result hit times failed", zap.String("cache_key", p.queryResultCacheKey), zap.Error(err))
		}

		return CachingStatusHit, result
	}

	return CachingStatusMiss, nil
}

func (c *Caching) addCachingResponseHeaders(s CachingStatus, r *cachingQueryResult, p *cachingPlan, h http.Header) {
	h.Set("x-cache", string(s))

	if s == CachingStatusPass {
		return
	}

	uniqueVaries := make(map[string]struct{})

	for name := range p.VaryNames {
		for v := range c.Varies[name].Headers {
			uniqueVaries[v] = struct{}{}
		}

		for v := range c.Varies[name].Cookies {
			uniqueVaries[fmt.Sprintf("cookie:%s", v)] = struct{}{}
		}
	}

	for vary := range uniqueVaries {
		h.Add("vary", vary)
	}

	if s == CachingStatusHit {
		age := int64(r.Age().Seconds())
		maxAge := int64(time.Duration(r.MaxAge).Seconds())
		cacheControl := []string{"public", fmt.Sprintf("s-maxage=%d", maxAge)}

		if r.Swr > 0 {
			swr := int64(time.Duration(r.Swr).Seconds())
			cacheControl = append(cacheControl, fmt.Sprintf("stale-while-revalidate=%d", swr))
		}

		h.Set("age", fmt.Sprintf("%d", age))
		h.Set("cache-control", strings.Join(cacheControl, "; "))
		h.Set("x-cache-hits", fmt.Sprintf("%d", r.HitTime))
	}

	if c.DebugHeaders {
		h.Set("x-debug-result-cache-key", p.queryResultCacheKey)

		if r == nil {
			return
		}

		if len(r.Tags.TypeKeys()) == 0 {
			h.Set("x-debug-result-missing-type-keys", "")
		}

		h.Set("x-debug-result-tags", strings.Join(r.Tags.ToSlice(), ", "))
	}
}

func (c *Caching) handleMutationRequest(w http.ResponseWriter, r *cachingRequest, h caddyhttp.HandlerFunc) (err error) {
	if !c.AutoInvalidate {
		return h(w, r.httpRequest)
	}

	bodyBuff := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(bodyBuff)
	bodyBuff.Reset()

	crw := newCachingResponseWriter(bodyBuff)
	err = h(crw, r.httpRequest)

	if err != nil {
		return err
	}

	defer func() {
		err = crw.WriteResponse(w)
	}()

	mt, _, _ := mime.ParseMediaType(crw.Header().Get("content-type"))

	if crw.Status() != http.StatusOK || mt != "application/json" {
		return err
	}

	foundTags := make(cachingTags)
	tagAnalyzer := newCachingTagAnalyzer(r, c.TypeKeys)

	if aErr := tagAnalyzer.AnalyzeResult(crw.buffer.Bytes(), nil, foundTags); aErr != nil {
		c.logger.Info("fail to analyze result tags", zap.Error(aErr))

		return err
	}

	purgeTags := foundTags.TypeKeys().ToSlice()

	if len(purgeTags) == 0 {
		return err
	}

	if c.DebugHeaders {
		w.Header().Set("x-debug-purging-tags", strings.Join(purgeTags, "; "))
	}

	go func() {
		if err := c.purgeQueryResultByTags(c.ctxBackground, purgeTags); err != nil {
			c.logger.Info("fail to purge query result by tags", zap.Error(err))
		}
	}()

	return err
}
