// Package gateway implements the domain gateway port against a remote Jupyter
// Gateway / Enterprise Gateway server. REST operations are typed; the kernel
// channels WebSocket is proxied frame-for-frame (see websocket_proxy.go).
package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	"go.uber.org/zap"
)

const (
	defaultConnectTimeout = 40 * time.Second
	defaultRequestTimeout = 42 * time.Second
	defaultMaxRetries     = 2
	maxRetriesCap         = 10

	kernelsEndpoint     = "/api/kernels"
	sessionsEndpoint    = "/api/sessions"
	kernelSpecsEndpoint = "/api/kernelspecs"
)

// Gateway is the concrete implementation of domain/gateway.Gateway.
type Gateway struct {
	httpTarget *url.URL
	wsTarget   *url.URL

	httpClient *http.Client
	wsDialer   *wsDialer

	authHeaderKey string
	authValue     string
	headers       map[string]string

	log *zap.Logger
}

// compile-time assertion that Gateway satisfies the domain port.
var _ domaingateway.Gateway = (*Gateway)(nil)

// NewOptionalGateway builds the gateway client from config. It returns
// (nil, nil) when cfg.URL is empty so callers can disable the feature
// gracefully, mirroring the NewOptional* pattern used elsewhere in the service.
func NewOptionalGateway(cfg appconfig.GatewayConfig, log *zap.Logger) (domaingateway.Gateway, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, nil
	}

	httpTarget, err := url.Parse(strings.TrimSpace(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("parse gateway url: %w", err)
	}
	if httpTarget.Scheme != "http" && httpTarget.Scheme != "https" {
		return nil, fmt.Errorf("gateway url must start with http(s): %q", cfg.URL)
	}

	wsTarget, err := resolveWSTarget(cfg.WSURL, httpTarget)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	connectTimeout := cfg.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = defaultConnectTimeout
	}
	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}
	maxRetries := cfg.MaxRequestRetries
	if maxRetries < 0 {
		maxRetries = defaultMaxRetries
	}
	if maxRetries > maxRetriesCap {
		maxRetries = maxRetriesCap
	}

	if log == nil {
		log = zap.NewNop()
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   connectTimeout,
		ResponseHeaderTimeout: requestTimeout,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}

	return &Gateway{
		httpTarget: httpTarget,
		wsTarget:   wsTarget,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &retryingTransport{
				base:       transport,
				maxRetries: maxRetries,
				log:        log,
			},
		},
		wsDialer: &wsDialer{
			tlsConfig:      tlsConfig,
			connectTimeout: connectTimeout,
		},
		authHeaderKey: authHeaderKey(cfg.AuthHeaderKey),
		authValue:     authValue(cfg.AuthScheme, cfg.AuthToken),
		headers:       cfg.Headers,
		log:           log,
	}, nil
}

// --- Sessions ---

func (g *Gateway) CreateSession(ctx context.Context, req domaingateway.CreateSessionRequest) (domaingateway.Session, error) {
	var out domaingateway.Session
	err := g.doJSON(ctx, http.MethodPost, sessionsEndpoint, req, &out)
	return out, err
}

func (g *Gateway) ListSessions(ctx context.Context) ([]domaingateway.Session, error) {
	var out []domaingateway.Session
	err := g.doJSON(ctx, http.MethodGet, sessionsEndpoint, nil, &out)
	return out, err
}

func (g *Gateway) GetSession(ctx context.Context, sessionID string) (domaingateway.Session, error) {
	var out domaingateway.Session
	err := g.doJSON(ctx, http.MethodGet, sessionPath(sessionID), nil, &out)
	return out, err
}

func (g *Gateway) UpdateSession(ctx context.Context, sessionID string, req domaingateway.UpdateSessionRequest) (domaingateway.Session, error) {
	var out domaingateway.Session
	err := g.doJSON(ctx, http.MethodPatch, sessionPath(sessionID), req, &out)
	return out, err
}

func (g *Gateway) DeleteSession(ctx context.Context, sessionID string) error {
	return g.doNoContent(ctx, http.MethodDelete, sessionPath(sessionID), nil)
}

// --- Kernels ---

func (g *Gateway) CreateKernel(ctx context.Context, req domaingateway.CreateKernelRequest) (domaingateway.Kernel, error) {
	var out domaingateway.Kernel
	err := g.doJSON(ctx, http.MethodPost, kernelsEndpoint, req, &out)
	return out, err
}

func (g *Gateway) ListKernels(ctx context.Context) ([]domaingateway.Kernel, error) {
	var out []domaingateway.Kernel
	err := g.doJSON(ctx, http.MethodGet, kernelsEndpoint, nil, &out)
	return out, err
}

func (g *Gateway) GetKernel(ctx context.Context, kernelID string) (domaingateway.Kernel, error) {
	var out domaingateway.Kernel
	err := g.doJSON(ctx, http.MethodGet, kernelPath(kernelID), nil, &out)
	return out, err
}

func (g *Gateway) DeleteKernel(ctx context.Context, kernelID string) error {
	return g.doNoContent(ctx, http.MethodDelete, kernelPath(kernelID), nil)
}

func (g *Gateway) InterruptKernel(ctx context.Context, kernelID string) error {
	return g.doNoContent(ctx, http.MethodPost, kernelPath(kernelID)+"/interrupt", struct{}{})
}

// RestartKernel restarts the kernel then re-fetches the model, matching
// jupyter_server which does not trust the restart response body.
func (g *Gateway) RestartKernel(ctx context.Context, kernelID string) (domaingateway.Kernel, error) {
	if err := g.doNoContent(ctx, http.MethodPost, kernelPath(kernelID)+"/restart", struct{}{}); err != nil {
		return domaingateway.Kernel{}, err
	}
	return g.GetKernel(ctx, kernelID)
}

// --- Kernelspecs ---

func (g *Gateway) ListKernelSpecs(ctx context.Context) (domaingateway.KernelSpecs, error) {
	var out domaingateway.KernelSpecs
	err := g.doJSON(ctx, http.MethodGet, kernelSpecsEndpoint, nil, &out)
	return out, err
}

func (g *Gateway) GetKernelSpec(ctx context.Context, name string) (domaingateway.KernelSpec, error) {
	var out domaingateway.KernelSpec
	err := g.doJSON(ctx, http.MethodGet, kernelSpecsEndpoint+"/"+url.PathEscape(name), nil, &out)
	return out, err
}

func (g *Gateway) GetKernelSpecResource(ctx context.Context, resourcePath string) (domaingateway.KernelSpecResource, error) {
	data, status, contentType, err := g.call(ctx, http.MethodGet, resourcePath, nil)
	if err != nil {
		return domaingateway.KernelSpecResource{}, err
	}
	if status < 200 || status >= 300 {
		return domaingateway.KernelSpecResource{}, &domaingateway.APIError{StatusCode: status, ContentType: contentType, Body: data}
	}
	return domaingateway.KernelSpecResource{ContentType: contentType, Data: data}, nil
}

// --- HTTP plumbing ---

// doJSON sends an optional JSON body and decodes a JSON response into out.
func (g *Gateway) doJSON(ctx context.Context, method, path string, body, out any) error {
	data, status, contentType, err := g.call(ctx, method, path, body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &domaingateway.APIError{StatusCode: status, ContentType: contentType, Body: data}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode gateway response: %w", err)
	}
	return nil
}

// doNoContent sends a request that is not expected to return a body. A 404 is
// treated as success so deletes/interrupts are idempotent (as in jupyter_server).
func (g *Gateway) doNoContent(ctx context.Context, method, path string, body any) error {
	data, status, contentType, err := g.call(ctx, method, path, body)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status < 200 || status >= 300 {
		return &domaingateway.APIError{StatusCode: status, ContentType: contentType, Body: data}
	}
	return nil
}

// call performs the HTTP round trip to the gateway and returns the raw body.
func (g *Gateway) call(ctx context.Context, method, path string, body any) ([]byte, int, string, error) {
	var reader io.Reader
	hasBody := body != nil
	if hasBody {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, "", fmt.Errorf("encode gateway request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	target := *g.httpTarget
	target.Path = singleJoiningSlash(g.httpTarget.Path, path)

	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, 0, "", fmt.Errorf("build gateway request: %w", err)
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	g.applyAuth(req.Header)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.log.Warn("gateway request failed",
			zap.String("method", method),
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, 0, "", fmt.Errorf("gateway request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("Content-Type"), fmt.Errorf("read gateway response: %w", err)
	}
	return data, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func (g *Gateway) applyAuth(h http.Header) {
	for k, v := range g.headers {
		h.Set(k, v)
	}
	if g.authValue != "" {
		h.Set(g.authHeaderKey, g.authValue)
	}
}

func (g *Gateway) authHeader() http.Header {
	h := http.Header{}
	g.applyAuth(h)
	return h
}

func sessionPath(sessionID string) string {
	return sessionsEndpoint + "/" + url.PathEscape(sessionID)
}

func kernelPath(kernelID string) string {
	return kernelsEndpoint + "/" + url.PathEscape(kernelID)
}

func authHeaderKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "Authorization"
	}
	return key
}

func authValue(scheme, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		return token
	}
	return scheme + " " + token
}

func resolveWSTarget(wsURL string, httpTarget *url.URL) (*url.URL, error) {
	wsURL = strings.TrimSpace(wsURL)
	if wsURL == "" {
		derived := *httpTarget
		switch httpTarget.Scheme {
		case "https":
			derived.Scheme = "wss"
		default:
			derived.Scheme = "ws"
		}
		return &derived, nil
	}

	parsed, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("parse gateway ws_url: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, fmt.Errorf("gateway ws_url must start with ws(s): %q", wsURL)
	}
	return parsed, nil
}

func buildTLSConfig(cfg appconfig.GatewayConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !cfg.ValidateCert, //nolint:gosec // validate_cert is operator-controlled
	}

	if cfg.CACerts != "" {
		pem, err := os.ReadFile(cfg.CACerts)
		if err != nil {
			return nil, fmt.Errorf("read gateway ca_certs: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse gateway ca_certs: no certificates in %q", cfg.CACerts)
		}
		tlsConfig.RootCAs = pool
	}

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("load gateway client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// retryingTransport retries idempotent (GET/DELETE) requests on transient
// gateway failures, matching jupyter_server's RetryableHTTPClient behavior.
type retryingTransport struct {
	base       http.RoundTripper
	maxRetries int
	log        *zap.Logger
}

func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	retriable := req.Method == http.MethodGet || req.Method == http.MethodDelete

	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; ; attempt++ {
		resp, err = t.base.RoundTrip(req)

		if !retriable || attempt >= t.maxRetries {
			return resp, err
		}
		if err == nil && !isRetriableStatus(resp.StatusCode) {
			return resp, nil
		}

		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		backoff := time.Duration(float64(time.Second) * 0.1 * math.Pow(2, float64(attempt)))
		t.log.Debug("retrying gateway request",
			zap.String("method", req.Method),
			zap.String("path", req.URL.Path),
			zap.Int("attempt", attempt+1),
			zap.Duration("backoff", backoff),
		)
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		}
	}
}

func isRetriableStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
