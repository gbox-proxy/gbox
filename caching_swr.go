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

type cachingSwrResponseWriter struct {
	header http.Header
	status int
	buffer *bytes.Buffer
}

func newCachingSwrResponseWriter(buffer *bytes.Buffer) *cachingSwrResponseWriter {
	return &cachingSwrResponseWriter{
		header: make(http.Header),
		buffer: buffer,
	}
}

func (c *cachingSwrResponseWriter) Status() int {
	return c.status
}

func (c *cachingSwrResponseWriter) Header() http.Header {
	return c.header
}

func (c *cachingSwrResponseWriter) Write(i []byte) (int, error) {
	return c.buffer.Write(i)
}

func (c *cachingSwrResponseWriter) WriteHeader(statusCode int) {
	c.status = statusCode
}

func (c *Caching) swrQueryResult(result *cachingQueryResult, request *cachingRequest, handler caddyhttp.HandlerFunc) error {
	buff := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buff)
	buff.Reset()
	rw := newCachingSwrResponseWriter(buff)

	if err := handler(rw, request.httpRequest); err != nil {
		return err
	}

	ct := rw.Header().Get("content-type")
	mt, _, _ := mime.ParseMediaType(ct)

	if rw.Status() != http.StatusOK || mt != "application/json" {
		return fmt.Errorf("getting invalid response from upstream, status: %d, content-type: %s", rw.Status(), ct)
	}

	if err := c.cachingQueryResult(request, result.plan, buff.Bytes(), rw.Header()); err != nil {
		return err
	}

	return nil
}

func newSwrHttpRequest(ctx context.Context, r *http.Request) *http.Request {
	rCtx := r.Context()
	ctx = context.WithValue(ctx, caddy.ReplacerCtxKey, rCtx.Value(caddy.ReplacerCtxKey))
	ctx = context.WithValue(ctx, caddyhttp.ServerCtxKey, rCtx.Value(caddyhttp.ServerCtxKey))
	ctx = context.WithValue(ctx, caddyhttp.VarsCtxKey, rCtx.Value(caddyhttp.VarsCtxKey))
	ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, rCtx.Value(caddyhttp.OriginalRequestCtxKey))

	return r.Clone(ctx)
}
