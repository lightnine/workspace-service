package router

import (
	"strings"

	apphandler "git.woa.com/leondli/workspace-service/internal/adapter/http/handler"
	appmiddleware "git.woa.com/leondli/workspace-service/internal/adapter/http/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func New(log *zap.Logger) *gin.Engine {
	return NewWithHandlers(log, "", nil, nil)
}

func NewWithHandlers(log *zap.Logger, urlPrefix string, gitHandler *apphandler.GitHandler, gatewayHandler *apphandler.GatewayHandler) *gin.Engine {
	engine := gin.New()
	engine.Use(appmiddleware.RequestID())
	engine.Use(appmiddleware.Recovery(log))

	healthHandler := apphandler.NewHealthHandler()
	engine.GET("/healthz", healthHandler.Healthz)
	engine.GET("/readyz", healthHandler.Readyz)

	if gitHandler != nil {
		engine.POST(joinRoute(urlPrefix, "/CloneRepo"), gitHandler.CloneRepository)
	}

	if gatewayHandler != nil {
		registerGatewayRoutes(engine, gatewayHandler)
	}

	return engine
}

// registerGatewayRoutes mounts the Jupyter-compatible REST + WebSocket surface
// proxied to the remote gateway. Paths follow the Jupyter API contract and are
// therefore not subject to the verb+noun url_prefix used by native endpoints.
func registerGatewayRoutes(engine *gin.Engine, h *apphandler.GatewayHandler) {
	engine.POST("/api/sessions", h.CreateSession)
	engine.GET("/api/sessions", h.ListSessions)
	engine.GET("/api/sessions/:session_id", h.GetSession)
	engine.PATCH("/api/sessions/:session_id", h.UpdateSession)
	engine.DELETE("/api/sessions/:session_id", h.DeleteSession)

	engine.POST("/api/kernels", h.CreateKernel)
	engine.GET("/api/kernels", h.ListKernels)
	engine.GET("/api/kernels/:kernel_id", h.GetKernel)
	engine.DELETE("/api/kernels/:kernel_id", h.DeleteKernel)
	engine.POST("/api/kernels/:kernel_id/interrupt", h.InterruptKernel)
	engine.POST("/api/kernels/:kernel_id/restart", h.RestartKernel)
	engine.GET("/api/kernels/:kernel_id/channels", h.Channels)

	engine.GET("/api/kernelspecs", h.ListKernelSpecs)
	engine.GET("/api/kernelspecs/:name", h.GetKernelSpec)
	engine.GET("/kernelspecs/*resource", h.GetKernelSpecResource)
}

func joinRoute(prefix, path string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return path
	}
	return "/" + strings.Trim(strings.TrimSuffix(prefix, "/")+"/"+strings.TrimPrefix(path, "/"), "/")
}
