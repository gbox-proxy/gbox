package gbox

import (
	"bytes"
	"io"
	"net/http"
)

type cachingResponseWriter struct {
	header http.Header
	status int
	buffer *bytes.Buffer
}

func newCachingResponseWriter(buffer *bytes.Buffer) *cachingResponseWriter {
	return &cachingResponseWriter{
		header: make(http.Header),
		buffer: buffer,
	}
}

func (c *cachingResponseWriter) Status() int {
	return c.status
}

func (c *cachingResponseWriter) Header() http.Header {
	return c.header
}

func (c *cachingResponseWriter) Write(i []byte) (int, error) {
	return c.buffer.Write(i)
}

func (c *cachingResponseWriter) WriteHeader(statusCode int) {
	c.status = statusCode
}

func (c *cachingResponseWriter) WriteResponse(dst http.ResponseWriter) (err error) {
	for h, v := range c.Header() {
		dst.Header()[h] = v
	}

	dst.WriteHeader(c.Status())

	_, err = io.Copy(dst, c.buffer)

	return
}
