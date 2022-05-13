package gbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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

func (h *Handler) onWsSubscribe(r *graphql.Request) (err error) {
	if err = normalizeGraphqlRequest(h.schema, r); err != nil {
		return err
	}

	if err = h.validateGraphqlRequest(r); err != nil {
		return err
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
	ID      interface{}     `json:"id"`
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

	for {
		msg := new(wsMessage)
		request := new(graphql.Request)
		data, _, e := wsutil.ReadClientData(buff)

		if e != nil {
			return n, err
		}

		if e = json.Unmarshal(data, msg); e != nil {
			continue
		}

		if msg.Type != "subscribe" && msg.Type != "start" {
			continue
		}

		if e = json.Unmarshal(msg.Payload, request); e != nil {
			continue
		}

		if e = c.onWsSubscribe(request); e != nil {
			c.writeErrorMessage(msg.ID, e)
			c.writeCompleteMessage(msg.ID)

			return n, io.EOF
		}

		c.request = request
		c.subscribeAt = time.Now()

		return n, err
	}
}

func (c *wsConn) writeErrorMessage(id interface{}, errMsg error) error {
	errMsgRaw, errMsgErr := json.Marshal(graphql.RequestErrorsFromError(errMsg))

	if errMsgErr != nil {
		return errMsgErr
	}

	msg := &wsMessage{
		ID:      id,
		Type:    "error",
		Payload: json.RawMessage(errMsgRaw),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return wsutil.WriteServerText(c, payload)
}

func (c *wsConn) writeCompleteMessage(id interface{}) error {
	msg := &wsMessage{
		ID:   id,
		Type: "complete",
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return wsutil.WriteServerText(c, payload)
}
