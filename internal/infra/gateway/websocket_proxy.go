package gateway

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"go.uber.org/zap"
)

// wsReadLimit disables the per-message read cap; Jupyter iopub output messages
// can exceed the library default of 32 KiB.
const wsReadLimit = -1

type wsDialer struct {
	tlsConfig      *tls.Config
	connectTimeout time.Duration
}

func (d *wsDialer) httpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: d.connectTimeout,
			}).DialContext,
			TLSHandshakeTimeout: d.connectTimeout,
			TLSClientConfig:     d.tlsConfig,
		},
	}
}

// ProxyWebSocket bridges the kernel channels WebSocket between the caller and
// the gateway. It dials the gateway first so the subprotocol the gateway
// negotiates is the exact one offered back to the caller.
func (g *Gateway) ProxyWebSocket(w http.ResponseWriter, r *http.Request) {
	requested := requestedSubprotocols(r)

	target := *g.wsTarget
	target.Path = singleJoiningSlash(g.wsTarget.Path, r.URL.Path)
	target.RawQuery = r.URL.RawQuery

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gwConn, _, err := websocket.Dial(ctx, target.String(), &websocket.DialOptions{
		HTTPClient:   g.wsDialer.httpClient(),
		HTTPHeader:   g.authHeader(),
		Subprotocols: requested,
	})
	if err != nil {
		g.log.Warn("gateway websocket dial failed",
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
		http.Error(w, "gateway websocket dial failed", http.StatusBadGateway)
		return
	}
	gwConn.SetReadLimit(wsReadLimit)
	defer gwConn.CloseNow() //nolint:errcheck

	acceptOpts := &websocket.AcceptOptions{
		// The proxy lives behind the service edge; origin enforcement happens
		// upstream, so accept cross-origin upgrades here.
		InsecureSkipVerify: true,
	}
	if negotiated := gwConn.Subprotocol(); negotiated != "" {
		acceptOpts.Subprotocols = []string{negotiated}
	}

	clientConn, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		g.log.Warn("client websocket accept failed",
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
		return
	}
	clientConn.SetReadLimit(wsReadLimit)
	defer clientConn.CloseNow() //nolint:errcheck

	errs := make(chan error, 2)
	go func() { errs <- pumpWS(ctx, gwConn, clientConn) }()
	go func() { errs <- pumpWS(ctx, clientConn, gwConn) }()

	err = <-errs
	cancel()

	code, reason := closeStatus(err)
	_ = clientConn.Close(code, reason)
	_ = gwConn.Close(code, reason)
}

// pumpWS copies frames from src to dst preserving the message type until an
// error (including a normal close) occurs.
func pumpWS(ctx context.Context, dst, src *websocket.Conn) error {
	for {
		typ, data, err := src.Read(ctx)
		if err != nil {
			return err
		}
		if err := dst.Write(ctx, typ, data); err != nil {
			return err
		}
	}
}

func closeStatus(err error) (websocket.StatusCode, string) {
	if err == nil {
		return websocket.StatusNormalClosure, ""
	}
	if status := websocket.CloseStatus(err); status != -1 {
		return status, ""
	}
	if errors.Is(err, context.Canceled) {
		return websocket.StatusNormalClosure, ""
	}
	return websocket.StatusInternalError, ""
}

// requestedSubprotocols parses the Sec-WebSocket-Protocol request header into
// the ordered list of subprotocols the client offered.
func requestedSubprotocols(r *http.Request) []string {
	var out []string
	for _, header := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(header, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func singleJoiningSlash(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	aSlash := a[len(a)-1] == '/'
	bSlash := b[0] == '/'
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	default:
		return a + b
	}
}
