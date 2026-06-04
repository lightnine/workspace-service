package gateway

import (
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailers":            {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

// ProxyHTTP relays an HTTP request to the configured gateway backend and streams
// the response back unchanged (status, headers, body).
func (g *Gateway) ProxyHTTP(w http.ResponseWriter, r *http.Request) {
	target := *g.httpTarget
	target.Path = singleJoiningSlash(g.httpTarget.Path, r.URL.Path)
	target.RawQuery = r.URL.RawQuery

	var body io.Reader = http.NoBody
	if r.Body != nil && r.ContentLength != 0 {
		body = r.Body
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), body)
	if err != nil {
		g.log.Warn("build gateway proxy request failed", zap.String("path", r.URL.Path), zap.Error(err))
		http.Error(w, "gateway proxy request failed", http.StatusBadGateway)
		return
	}

	copyForwardHeaders(req.Header, r.Header)
	g.applyAuth(req.Header)
	if host := r.Host; host != "" {
		req.Host = host
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.log.Warn("gateway proxy request failed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
		http.Error(w, "gateway proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyForwardHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		g.log.Warn("gateway proxy response copy failed",
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
	}
}

func copyForwardHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	_, ok := hopByHopHeaders[strings.ToLower(key)]
	return ok
}
