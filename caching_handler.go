package gbox

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"go.uber.org/zap"
	"mime"
	"net/http"
	"strings"
	"time"
)

var handleUnknownOperationTypeError = errors.New("unknown operation type")

// HandleRequest caching GraphQL query result by configured rules and varies.
func (c *Caching) HandleRequest(w http.ResponseWriter, r *cachingRequest, h caddyhttp.HandlerFunc) error {
	// Remove `accept-encoding` header to prevent response body encoded when forward request to upstream
	// encode directive had read this header, safe to delete it.
	r.httpRequest.Header.Del("accept-encoding")
	operationType, _ := r.gqlRequest.OperationType()

	switch operationType {
	case graphql.OperationTypeQuery:
		return c.handleQueryRequest(w, r, h)
	case graphql.OperationTypeMutation:
		return c.handleMutationRequest(w, r, h)
	}

	return handleUnknownOperationTypeError
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

	switch status {
	case CachingStatusMiss:
		defer c.addMetricsCacheMiss(r.gqlRequest)

		recordBuff := bufferPool.Get().(*bytes.Buffer)
		defer bufferPool.Put(recordBuff)
		recordBuff.Reset()

		recorder := caddyhttp.NewResponseRecorder(w, recordBuff, func(httpStatus int, header http.Header) bool {
			var shouldBuffer bool
			mt, _, _ := mime.ParseMediaType(header.Get("content-type"))

			if httpStatus == http.StatusOK && mt == "application/json" {
				// respect no-store directive
				shouldBuffer = r.cacheControl == nil || !r.cacheControl.NoStore
			}

			if !shouldBuffer {
				// when should not record response we need to add caching header first
				c.addCachingResponseHeaders(status, result, plan, header)
			}

			return shouldBuffer
		})

		defer func() {
			if !recorder.Buffered() {
				return
			}

			defer func() {
				c.addCachingResponseHeaders(status, result, plan, recorder.Header())
				err = recorder.WriteResponse()
			}()

			header := recorder.Header().Clone()
			bodyBuff := bufferPool.Get().(*bytes.Buffer)
			bodyBuff.Reset()
			bodyBuff.Write(recordBuff.Bytes())

			go func() {
				defer bufferPool.Put(bodyBuff)
				err := c.cachingQueryResult(r, plan, bodyBuff.Bytes(), header)

				if err != nil {
					c.logger.Info("fail to cache query result", zap.Error(err))
				} else {
					c.logger.Info("caching query result successful", zap.String("cache_key", plan.queryResultCacheKey))
				}
			}()
		}()

		err = h(recorder, r.httpRequest)
	case CachingStatusHit:
		defer c.addMetricsCacheHit(r.gqlRequest)

		for header, values := range result.Header {
			w.Header()[header] = values
		}

		c.addCachingResponseHeaders(status, result, plan, w.Header())
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(result.Body)

		if result.Status() != CachingQueryResultStale {
			return nil
		}

		r.httpRequest = prepareSwrHttpRequest(c.ctxBackground, r.httpRequest, w)

		go func() {
			if err := c.swrQueryResult(result, r, h); err != nil {
				c.logger.Info("swr failed, can not update query result", zap.String("cache_key", plan.queryResultCacheKey), zap.Error(err))
			} else {
				c.logger.Info("swr query result successful", zap.String("cache_key", plan.queryResultCacheKey))
			}
		}()
	case CachingStatusPass:
		defer c.addMetricsCachePass(r.gqlRequest)
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
		c.increaseQueryResultHitTimes(r.httpRequest.Context(), result)

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
		cacheControl := []string{"public"}

		if r.MaxAge != nil {
			maxAge := int64(time.Duration(*r.MaxAge).Seconds())
			cacheControl = append(cacheControl, fmt.Sprintf("s-maxage=%d", maxAge))
		}

		if r.Swr != nil {
			swr := int64(time.Duration(*r.Swr).Seconds())
			cacheControl = append(cacheControl, fmt.Sprintf("stale-while-revalidate=%d", swr))
		}

		h.Set("age", fmt.Sprintf("%d", age))
		h.Set("cache-control", strings.Join(cacheControl, "; "))
		h.Set("x-cache-hits", fmt.Sprintf("%d", r.HitTime))
	}
}

func (c *Caching) handleMutationRequest(w http.ResponseWriter, r *cachingRequest, h caddyhttp.HandlerFunc) (err error) {
	if !c.AutoInvalidate {
		return h(w, r.httpRequest)
	}

	recordBuff := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(recordBuff)
	recordBuff.Reset()

	recorder := caddyhttp.NewResponseRecorder(w, recordBuff, func(status int, header http.Header) bool {
		mt, _, _ := mime.ParseMediaType(header.Get("content-type"))

		return status == http.StatusOK && mt == "application/json"
	})

	defer func() {
		if !recorder.Buffered() {
			return
		}

		bodyBuff := bufferPool.Get().(*bytes.Buffer)
		bodyBuff.Reset()
		bodyBuff.Write(recordBuff.Bytes())

		go func() {
			defer bufferPool.Put(bodyBuff)

			err := c.purgeQueryResultByMutationResult(r, bodyBuff.Bytes())

			if err != nil {
				c.logger.Info("fail to purge query result", zap.Error(err))
			}
		}()

		err = recorder.WriteResponse()
	}()

	err = h(recorder, r.httpRequest)

	return
}
