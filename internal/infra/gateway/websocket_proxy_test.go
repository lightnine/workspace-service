package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	"github.com/coder/websocket"
)

func TestProxyWebSocketBridgesFrames(t *testing.T) {
	t.Parallel()

	// Gateway side: echo server that also reports the negotiated subprotocol.
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: requestedSubprotocols(r),
		})
		if err != nil {
			return
		}
		conn.SetReadLimit(wsReadLimit)
		defer conn.CloseNow()

		ctx := context.Background()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, typ, append([]byte("echo:"), data...)); err != nil {
				return
			}
		}
	}))
	defer gateway.Close()

	gw, err := NewOptionalGateway(appconfig.GatewayConfig{URL: gateway.URL}, nil)
	if err != nil {
		t.Fatalf("build gateway: %v", err)
	}

	proxySrv := httptest.NewServer(http.HandlerFunc(gw.ProxyWebSocket))
	defer proxySrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := strings.Replace(proxySrv.URL, "http", "ws", 1) + "/api/kernels/k1/channels?session_id=s1"
	clientConn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"v1.kernel.websocket.jupyter.org"},
	})
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer clientConn.CloseNow()

	if got := clientConn.Subprotocol(); got != "v1.kernel.websocket.jupyter.org" {
		t.Fatalf("subprotocol = %q, want negotiated jupyter subprotocol", got)
	}

	if err := clientConn.Write(ctx, websocket.MessageText, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}

	typ, data, err := clientConn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("message type = %v, want text", typ)
	}
	if string(data) != "echo:hello" {
		t.Fatalf("data = %q, want echo:hello", string(data))
	}

	clientConn.Close(websocket.StatusNormalClosure, "")
}
