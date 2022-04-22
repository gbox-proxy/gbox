package gbox

import (
	"bytes"
	"context"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"mime"
	"net/http"
)

func (c *Caching) swrQueryResult(ctx context.Context, result *cachingQueryResult, request *cachingRequest, handler caddyhttp.HandlerFunc) error {
	buff := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buff)
	buff.Reset()
	rw := newCachingResponseWriter(buff)

	if err := handler(rw, request.httpRequest); err != nil {
		return err
	}

	ct := rw.Header().Get("content-type")
	mt, _, _ := mime.ParseMediaType(ct)

	if rw.Status() != http.StatusOK || mt != "application/json" {
		return fmt.Errorf("getting invalid response from upstream, status: %d, content-type: %s", rw.Status(), ct)
	}

	if err := c.cachingQueryResult(ctx, request, result.plan, buff.Bytes(), rw.Header()); err != nil {
		return err
	}

	return nil
}

func prepareSwrHttpRequest(ctx context.Context, r *http.Request, w http.ResponseWriter) *http.Request {
	s := r.Context().Value(caddyhttp.ServerCtxKey).(*caddyhttp.Server)

	return caddyhttp.PrepareRequest(r.Clone(ctx), caddy.NewReplacer(), w, s)
}
