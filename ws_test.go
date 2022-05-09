package gbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gobwas/ws/wsutil"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/stretchr/testify/require"
)

type testWsSubscriber struct {
	t *testing.T
	r *graphql.Request
	d time.Duration
	e error
}

func (t *testWsSubscriber) onWsSubscribe(request *graphql.Request) error {
	t.r = request

	return t.e
}

func (t *testWsSubscriber) onWsClose(request *graphql.Request, duration time.Duration) {
	require.Equal(t.t, t.r, request)
	t.d = duration
}

type testWsResponseWriter struct {
	http.ResponseWriter
	wsConnBuff *bytes.Buffer
}

type testWsConn struct {
	net.Conn
	buffer *bytes.Buffer
}

func (c *testWsConn) Read(b []byte) (n int, err error) {
	if b == nil {
		return 0, errors.New("end")
	}

	return len(b), nil
}

func (c *testWsConn) Write(b []byte) (n int, err error) {
	return c.buffer.Write(b)
}

func (t *testWsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return &testWsConn{
		buffer: t.wsConnBuff,
	}, nil, nil
}

func newTestWsSubscriber(t *testing.T, err error) *testWsSubscriber {
	t.Helper()

	return &testWsSubscriber{
		t: t,
		e: err,
	}
}

func TestWsMetricsConn(t *testing.T) {
	s := newTestWsSubscriber(t, nil)
	w := newWebsocketResponseWriter(&testWsResponseWriter{wsConnBuff: new(bytes.Buffer)}, s)
	conn, _, _ := w.Hijack()
	buff := new(bytes.Buffer)
	wsutil.WriteClientText(buff, []byte(`{"type": "start", "payload":{"query": "subscription { users { id } }"}}`))

	n, err := conn.Read(buff.Bytes()) // subscribe

	require.NoError(t, err)
	require.Greater(t, n, 0)
	require.NotNil(t, s.r)
	require.Equal(t, s.d, time.Duration(0))

	conn.Read(nil) // end
	require.Greater(t, s.d, time.Duration(0))
}

func TestWsMetricsConnBadCases(t *testing.T) {
	testCases := map[string]struct {
		message string
		err     error
	}{
		"invalid_json": {
			message: `invalid`,
		},
		"invalid_struct": {
			message: `{}`,
		},
		"invalid_message_payload": {
			message: `{"type": "start", "payload": "invalid"}`,
		},
		"invalid_ws_message": {},
		"invalid_query": {
			message: `{"type": "start", "payload": {"query": "query { user { id } }"}}`,
			err:     errors.New("test"),
		},
	}

	for name, testCase := range testCases {
		wsConnBuff := new(bytes.Buffer)
		s := newTestWsSubscriber(t, testCase.err)
		w := newWebsocketResponseWriter(&testWsResponseWriter{wsConnBuff: wsConnBuff}, s)
		conn, _, _ := w.Hijack()
		buff := new(bytes.Buffer)

		if testCase.message != "invalid_ws_message" {
			wsutil.WriteClientText(buff, []byte(testCase.message))
		} else {
			buff.Write([]byte(name))
		}

		n, err := conn.Read(buff.Bytes())

		require.Equalf(t, err, s.e, "case %s: unexpected error", name)
		require.Greaterf(t, n, 0, "case %s: read bytes should greater than 0", name)
		require.Equalf(t, s.d, time.Duration(0), "case %s: duration should be 0", name)

		if s.e == nil {
			require.Nilf(t, s.r, "case %s: request should be nil", name)
		} else {
			require.NotNilf(t, s.r, "case %s: request should not be nil", name)
			data, _ := wsutil.ReadServerText(wsConnBuff)
			msg := &wsMessage{}
			json.Unmarshal(data, msg)

			require.Equalf(t, "error", msg.Type, "case %s: unexpected error type", name)

			data, _ = wsutil.ReadServerText(wsConnBuff)
			msg = &wsMessage{}
			json.Unmarshal(data, msg)

			require.Equalf(t, "complete", msg.Type, "case %s: msg type should be complete, but got %s", name, msg.Type)
		}
	}
}
