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

type wsSubscriber interface {
	onWsSubscribe(*graphql.Request) error
	onWsClose(*graphql.Request, time.Duration)
}

func (h *Handler) onWsSubscribe(r *graphql.Request) error {
	if err := normalizeGraphqlRequest(h.schema, r); err != nil {
		return err
	}

	isIntrospection, _ := r.IsIntrospectionQuery()

	if isIntrospection && h.DisabledIntrospection {
		return ErrNotAllowIntrospectionQuery
	}

	if h.Complexity != nil {
		requestErrors := h.Complexity.validateRequest(h.schema, r)

		if requestErrors.Count() > 0 {
			return requestErrors
		}
	}

	h.addMetricsBeginRequest(r)

	return nil
}

func (h *Handler) onWsClose(r *graphql.Request, d time.Duration) {
	h.addMetricsEndRequest(r, d)
}

type wsResponseWriter struct {
	*caddyhttp.ResponseWriterWrapper
	subscriber wsSubscriber
}

func newWebsocketResponseWriter(w http.ResponseWriter, s wsSubscriber) *wsResponseWriter {
	return &wsResponseWriter{
		ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{
			ResponseWriter: w,
		},
		subscriber: s,
	}
}

// Hijack connection for validating, and collecting metrics.
func (r *wsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, w, e := r.ResponseWriterWrapper.Hijack()

	if c != nil {
		c = &wsConn{
			Conn:         c,
			wsSubscriber: r.subscriber,
		}
	}

	return c, w, e
}

type wsConn struct {
	net.Conn
	wsSubscriber
	request     *graphql.Request
	subscribeAt time.Time
}

type wsMessage struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (c *wsConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)

	if c.request != nil && err != nil {
		c.onWsClose(c.request, time.Since(c.subscribeAt))
		c.request = nil
	}

	if c.request != nil || err != nil {
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
	msg := &wsMessage{}

	if e := decoder.Decode(msg); e != nil {
		return n, err
	}

	if msg.Type == "subscribe" || msg.Type == "start" {
		request := new(graphql.Request)

		if e := json.Unmarshal(msg.Payload, request); e != nil {
			return n, err
		}

		if err = c.onWsSubscribe(request); err != nil {
			msg = &wsMessage{
				ID:   msg.ID,
				Type: "error",
			}
			rawMsgPayload, _ := json.Marshal(graphql.RequestErrorsFromError(err)) //nolint:errchkjson
			msg.Payload = json.RawMessage(rawMsgPayload)
			payload, _ := json.Marshal(msg) //nolint:errchkjson
			wsutil.WriteServerText(c, payload)

			msg = &wsMessage{
				ID:   msg.ID,
				Type: "complete",
			}
			payload, _ = json.Marshal(msg) //nolint:errchkjson
			wsutil.WriteServerText(c, payload)

			return n, err
		}

		c.request = request
		c.subscribeAt = time.Now()
	}

	return n, err
}
