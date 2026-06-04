package handler

import (
	"errors"
	"net/http"

	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	"github.com/gin-gonic/gin"
)

// GatewayHandler exposes the Jupyter REST surface (sessions, kernels,
// kernelspecs) and the kernel channels WebSocket, fulfilling each operation via
// the gateway port. Responses use the raw Jupyter models for client
// compatibility rather than the service's {common,data} envelope.
type GatewayHandler struct {
	gateway domaingateway.Gateway
}

func NewGatewayHandler(gateway domaingateway.Gateway) *GatewayHandler {
	return &GatewayHandler{gateway: gateway}
}

// --- Sessions ---

func (h *GatewayHandler) CreateSession(c *gin.Context) {
	var req domaingateway.CreateSessionRequest
	if !bindJSON(c, &req) {
		return
	}
	session, err := h.gateway.CreateSession(c.Request.Context(), req)
	h.respond(c, http.StatusCreated, session, err)
}

func (h *GatewayHandler) ListSessions(c *gin.Context) {
	sessions, err := h.gateway.ListSessions(c.Request.Context())
	h.respond(c, http.StatusOK, sessions, err)
}

func (h *GatewayHandler) GetSession(c *gin.Context) {
	session, err := h.gateway.GetSession(c.Request.Context(), c.Param("session_id"))
	h.respond(c, http.StatusOK, session, err)
}

func (h *GatewayHandler) UpdateSession(c *gin.Context) {
	var req domaingateway.UpdateSessionRequest
	if !bindJSON(c, &req) {
		return
	}
	session, err := h.gateway.UpdateSession(c.Request.Context(), c.Param("session_id"), req)
	h.respond(c, http.StatusOK, session, err)
}

func (h *GatewayHandler) DeleteSession(c *gin.Context) {
	err := h.gateway.DeleteSession(c.Request.Context(), c.Param("session_id"))
	h.respondNoContent(c, err)
}

// --- Kernels ---

func (h *GatewayHandler) CreateKernel(c *gin.Context) {
	var req domaingateway.CreateKernelRequest
	if !bindJSONAllowEmpty(c, &req) {
		return
	}
	kernel, err := h.gateway.CreateKernel(c.Request.Context(), req)
	h.respond(c, http.StatusCreated, kernel, err)
}

func (h *GatewayHandler) ListKernels(c *gin.Context) {
	kernels, err := h.gateway.ListKernels(c.Request.Context())
	h.respond(c, http.StatusOK, kernels, err)
}

func (h *GatewayHandler) GetKernel(c *gin.Context) {
	kernel, err := h.gateway.GetKernel(c.Request.Context(), c.Param("kernel_id"))
	h.respond(c, http.StatusOK, kernel, err)
}

func (h *GatewayHandler) DeleteKernel(c *gin.Context) {
	err := h.gateway.DeleteKernel(c.Request.Context(), c.Param("kernel_id"))
	h.respondNoContent(c, err)
}

func (h *GatewayHandler) InterruptKernel(c *gin.Context) {
	err := h.gateway.InterruptKernel(c.Request.Context(), c.Param("kernel_id"))
	h.respondNoContent(c, err)
}

func (h *GatewayHandler) RestartKernel(c *gin.Context) {
	kernel, err := h.gateway.RestartKernel(c.Request.Context(), c.Param("kernel_id"))
	h.respond(c, http.StatusOK, kernel, err)
}

// --- Kernelspecs ---

func (h *GatewayHandler) ListKernelSpecs(c *gin.Context) {
	specs, err := h.gateway.ListKernelSpecs(c.Request.Context())
	h.respond(c, http.StatusOK, specs, err)
}

func (h *GatewayHandler) GetKernelSpec(c *gin.Context) {
	spec, err := h.gateway.GetKernelSpec(c.Request.Context(), c.Param("name"))
	h.respond(c, http.StatusOK, spec, err)
}

func (h *GatewayHandler) GetKernelSpecResource(c *gin.Context) {
	resource, err := h.gateway.GetKernelSpecResource(c.Request.Context(), "/kernelspecs"+c.Param("resource"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	contentType := resource.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, resource.Data)
}

// --- Kernel channels (WebSocket) ---

func (h *GatewayHandler) Channels(c *gin.Context) {
	h.gateway.ProxyWebSocket(c.Writer, c.Request)
}

// --- helpers ---

func (h *GatewayHandler) respond(c *gin.Context, status int, payload any, err error) {
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(status, payload)
}

func (h *GatewayHandler) respondNoContent(c *gin.Context, err error) {
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeError relays the gateway's own status code and error body when available,
// otherwise reports a 502. Error bodies follow Jupyter's {message,reason} shape.
func (h *GatewayHandler) writeError(c *gin.Context, err error) {
	var apiErr *domaingateway.APIError
	if errors.As(err, &apiErr) {
		contentType := apiErr.ContentType
		if contentType == "" {
			contentType = "application/json"
		}
		c.Data(apiErr.StatusCode, contentType, apiErr.Body)
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"message": "gateway request failed",
		"reason":  err.Error(),
	})
}

func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body", "reason": err.Error()})
		return false
	}
	return true
}

// bindJSONAllowEmpty binds a JSON body but tolerates an empty request, since
// POST /api/kernels may be sent with no body to use the default kernel.
func bindJSONAllowEmpty(c *gin.Context, out any) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true
	}
	return bindJSON(c, out)
}
