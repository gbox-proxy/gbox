package gbox

import (
	"bufio"
	"bytes"
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
}

func (t *testWsSubscriber) onWsSubscribe(request *graphql.Request) error {
	t.r = request

	return nil
}

func (t *testWsSubscriber) onWsClose(request *graphql.Request, duration time.Duration) {
	require.Equal(t.t, t.r, request)
	t.d = duration
}

type testWsResponseWriter struct {
	http.ResponseWriter
}

type testWsConn struct {
	net.Conn
}

func (c *testWsConn) Read(b []byte) (n int, err error) {
	if b == nil {
		return 0, errors.New("end")
	}

	return len(b), nil
}

func (t testWsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return &testWsConn{}, nil, nil
}

func newTestWsSubscriber(t *testing.T) *testWsSubscriber {
	t.Helper()

	return &testWsSubscriber{
		t: t,
	}
}

func TestWsMetricsConn(t *testing.T) {
	s := newTestWsSubscriber(t)
	w := newWebsocketResponseWriter(&testWsResponseWriter{}, s)
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
	}

	for name, testCase := range testCases {
		s := newTestWsSubscriber(t)
		w := newWebsocketResponseWriter(&testWsResponseWriter{}, s)
		conn, _, _ := w.Hijack()
		buff := new(bytes.Buffer)

		if testCase.message != "invalid_ws_message" {
			wsutil.WriteClientText(buff, []byte(testCase.message))
		} else {
			buff.Write([]byte(name))
		}

		n, err := conn.Read(buff.Bytes())

		require.NoErrorf(t, err, "case %s: should not error", name)
		require.Greaterf(t, n, 0, "case %s: read bytes should greater than 0", name)
		require.Nilf(t, s.r, "case %s: request should be nil", name)
		require.Equal(t, s.d, time.Duration(0), "case %s: duration should be 0", name)
	}
}
