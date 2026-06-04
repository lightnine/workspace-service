// Package gateway defines the port for talking to a remote Jupyter Gateway
// (Enterprise Gateway) server. It mirrors jupyter_server's manager layer:
// explicit operations (create kernel, create session, ...) that the infra
// implementation fulfils by calling the gateway's REST API, plus a streaming
// proxy for the kernel channels WebSocket.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Kernel is the Jupyter kernel model returned by the gateway.
type Kernel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LastActivity   string `json:"last_activity,omitempty"`
	ExecutionState string `json:"execution_state,omitempty"`
	Connections    int    `json:"connections,omitempty"`
}

// Session is the Jupyter session model returned by the gateway.
type Session struct {
	ID     string  `json:"id"`
	Path   string  `json:"path"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Kernel *Kernel `json:"kernel,omitempty"`
}

// KernelRef references a kernel by id and/or kernelspec name in session bodies.
type KernelRef struct {
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	CustomEnvs json.RawMessage `json:"custom_envs,omitempty"`
}

// KernelSpec is a single kernelspec entry. Its nested spec is kept raw because
// its shape varies by kernel and is not interpreted here.
type KernelSpec struct {
	Name      string            `json:"name"`
	Spec      json.RawMessage   `json:"spec,omitempty"`
	Resources map[string]string `json:"resources,omitempty"`
}

// KernelSpecs is the response of GET /api/kernelspecs.
type KernelSpecs struct {
	Default     string                `json:"default"`
	KernelSpecs map[string]KernelSpec `json:"kernelspecs"`
}

// CreateKernelRequest is the body of POST /api/kernels.
type CreateKernelRequest struct {
	Name string            `json:"name,omitempty"`
	Path string            `json:"path,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

// CreateSessionRequest is the body of POST /api/sessions.
type CreateSessionRequest struct {
	Path    string     `json:"path,omitempty"`
	Name    string     `json:"name,omitempty"`
	Type    string     `json:"type,omitempty"`
	Cluster string     `json:"cluster,omitempty"`
	Kernel  *KernelRef `json:"kernel,omitempty"`
}

// UpdateSessionRequest is the body of PATCH /api/sessions/{id}. Pointer fields
// distinguish "absent" from "set to empty" for partial updates.
type UpdateSessionRequest struct {
	Path   *string    `json:"path,omitempty"`
	Name   *string    `json:"name,omitempty"`
	Type   *string    `json:"type,omitempty"`
	Kernel *KernelRef `json:"kernel,omitempty"`
}

// KernelSpecResource is a raw kernelspec asset (e.g. a logo) plus its content type.
type KernelSpecResource struct {
	ContentType string
	Data        []byte
}

// Client is the set of gateway REST operations, modelled after jupyter_server's
// GatewayKernelManager / GatewaySessionManager / GatewayKernelSpecManager.
type Client interface {
	CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	UpdateSession(ctx context.Context, sessionID string, req UpdateSessionRequest) (Session, error)
	DeleteSession(ctx context.Context, sessionID string) error

	CreateKernel(ctx context.Context, req CreateKernelRequest) (Kernel, error)
	ListKernels(ctx context.Context) ([]Kernel, error)
	GetKernel(ctx context.Context, kernelID string) (Kernel, error)
	DeleteKernel(ctx context.Context, kernelID string) error
	InterruptKernel(ctx context.Context, kernelID string) error
	RestartKernel(ctx context.Context, kernelID string) (Kernel, error)

	ListKernelSpecs(ctx context.Context) (KernelSpecs, error)
	GetKernelSpec(ctx context.Context, name string) (KernelSpec, error)
	GetKernelSpecResource(ctx context.Context, resourcePath string) (KernelSpecResource, error)
}

// JupyterProxy forwards Jupyter-compatible HTTP and WebSocket traffic to the
// configured execution backend (wedata-jupyter-server or Enterprise Gateway).
// Extension routes (execute_task, spark stage, save_outputs, ...) are not
// modelled as typed operations; they are relayed verbatim like jupyter_server.
type JupyterProxy interface {
	ProxyHTTP(w http.ResponseWriter, r *http.Request)
	ProxyWebSocket(w http.ResponseWriter, r *http.Request)
}

// ChannelProxy is an alias for kernel channels WebSocket proxying.
type ChannelProxy interface {
	ProxyWebSocket(w http.ResponseWriter, r *http.Request)
}

// Gateway is the full port consumed by the HTTP adapter.
type Gateway interface {
	Client
	JupyterProxy
}

// APIError carries a non-2xx response from the gateway so the adapter can relay
// the gateway's own status code and error body back to the caller verbatim.
type APIError struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gateway returned status %d", e.StatusCode)
}
