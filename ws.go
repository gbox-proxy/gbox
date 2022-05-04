package gbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/gobwas/ws/wsutil"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
)

type wsMetricsResponseWriter struct {
	requestMetrics
	*caddyhttp.ResponseWriterWrapper
	schema *graphql.Schema
}

func newWebsocketMetricsResponseWriter(w http.ResponseWriter, s *graphql.Schema, rm requestMetrics) *wsMetricsResponseWriter {
	return &wsMetricsResponseWriter{
		ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{
			ResponseWriter: w,
		},
		schema:         s,
		requestMetrics: rm,
	}
}

// Hijack connection for collecting subscription metrics.
func (r *wsMetricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, w, e := r.ResponseWriterWrapper.Hijack()

	if c != nil {
		c = &wsMetricsConn{
			Conn:           c,
			requestMetrics: r.requestMetrics,
			schema:         r.schema,
		}
	}

	return c, w, e
}

type wsMetricsConn struct {
	net.Conn
	requestMetrics
	request     *graphql.Request
	schema      *graphql.Schema
	subscribeAt time.Time
}

func (c *wsMetricsConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)

	if c.request != nil || err != nil {
		if err != nil {
			c.addMetricsEndRequest(c.request, time.Since(c.subscribeAt))
			c.request = nil
		}

		return n, err
	}

	buff := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buff)
	buff.Reset()
	buff.Write(b[:n])

	r := wsutil.NewServerSideReader(buff)

	if _, e := r.NextFrame(); e != nil {
		return n, err
	}

	decoder := json.NewDecoder(r)
	msg := &struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}{}

	if e := decoder.Decode(msg); e != nil {
		return n, err
	}

	if msg.Type == "subscribe" || msg.Type == "start" {
		request := new(graphql.Request)

		if e := json.Unmarshal(msg.Payload, request); e != nil {
			return n, err
		}

		if e := normalizeGraphqlRequest(c.schema, request); e != nil {
			return n, err
		}

		c.request = request
		c.subscribeAt = time.Now()
		c.addMetricsBeginRequest(request)
	}

	return n, err
}
