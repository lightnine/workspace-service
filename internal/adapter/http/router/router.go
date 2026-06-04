package router

import (
	"strings"

	apphandler "git.woa.com/leondli/workspace-service/internal/adapter/http/handler"
	appmiddleware "git.woa.com/leondli/workspace-service/internal/adapter/http/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// New builds a Gin engine with health routes only. Prefer NewWithHandlers for production.
func New(log *zap.Logger) *gin.Engine {
	return NewWithHandlers(log, "", nil)
}

// NewWithHandlers registers routes for the given handler bundle. Pass nil to skip
// optional feature routes (e.g. gateway disabled).
func NewWithHandlers(log *zap.Logger, urlPrefix string, handlers *Handlers) *gin.Engine {
	engine := gin.New()
	engine.Use(appmiddleware.RequestID())
	engine.Use(appmiddleware.Recovery(log))

	healthHandler := apphandler.NewHealthHandler()
	engine.GET("/healthz", healthHandler.Healthz)
	engine.GET("/readyz", healthHandler.Readyz)

	if handlers != nil {
		registerFileRoutes(engine, urlPrefix, handlers.File)
		registerGitRoutes(engine, urlPrefix, handlers.Git)
		if handlers.Gateway != nil {
			registerGatewayRoutes(engine, handlers.Gateway)
		}
	}

	return engine
}

func registerFileRoutes(engine *gin.Engine, urlPrefix string, h *apphandler.FileHandler) {
	if h == nil {
		return
	}
	engine.POST(joinRoute(urlPrefix, "/CreateFolder"), h.CreateFolder)
	engine.POST(joinRoute(urlPrefix, "/CreateFile"), h.CreateFile)
	engine.POST(joinRoute(urlPrefix, "/DeletePath"), h.DeletePath)
	engine.POST(joinRoute(urlPrefix, "/MovePath"), h.MovePath)
	engine.POST(joinRoute(urlPrefix, "/CopyPath"), h.CopyPath)
	engine.POST(joinRoute(urlPrefix, "/RenamePath"), h.RenamePath)
	engine.POST(joinRoute(urlPrefix, "/ListFiles"), h.ListFiles)
	engine.POST(joinRoute(urlPrefix, "/ValidatePath"), h.ValidatePath)
	engine.POST(joinRoute(urlPrefix, "/GetFolderNodePath"), h.GetFolderNodePath)
	engine.POST(joinRoute(urlPrefix, "/ListRecycleBin"), h.ListRecycleBin)
	engine.POST(joinRoute(urlPrefix, "/RestorePath"), h.RestorePath)
	engine.POST(joinRoute(urlPrefix, "/EmptyRecycleBin"), h.EmptyRecycleBin)
	engine.POST(joinRoute(urlPrefix, "/GetFileInfo"), h.GetFileInfo)
	engine.POST(joinRoute(urlPrefix, "/ReadFile"), h.ReadFile)
	engine.POST(joinRoute(urlPrefix, "/WriteFile"), h.WriteFile)
	engine.POST(joinRoute(urlPrefix, "/DownloadFile"), h.DownloadFile)
}

func registerGitRoutes(engine *gin.Engine, urlPrefix string, h *apphandler.GitHandler) {
	if h == nil {
		return
	}
	engine.POST(joinRoute(urlPrefix, "/CloneRepo"), h.CloneRepository)
	engine.POST(joinRoute(urlPrefix, "/CreateGitFolder"), h.CreateGitFolder)
	engine.POST(joinRoute(urlPrefix, "/GetGitFolderStatus"), h.GetGitFolderStatus)
	engine.POST(joinRoute(urlPrefix, "/PullRepo"), h.PullRepo)
	engine.POST(joinRoute(urlPrefix, "/StageFiles"), h.StageFiles)
	engine.POST(joinRoute(urlPrefix, "/UnstageFiles"), h.UnstageFiles)
	engine.POST(joinRoute(urlPrefix, "/Commit"), h.Commit)
	engine.POST(joinRoute(urlPrefix, "/PushRepo"), h.PushRepo)
	engine.POST(joinRoute(urlPrefix, "/CommitAndPush"), h.CommitAndPush)
	engine.POST(joinRoute(urlPrefix, "/CreateBranch"), h.CreateBranch)
	engine.POST(joinRoute(urlPrefix, "/CheckoutBranch"), h.CheckoutBranch)
	engine.POST(joinRoute(urlPrefix, "/ListBranches"), h.ListBranches)
	engine.POST(joinRoute(urlPrefix, "/GetStatus"), h.GetStatus)
	engine.POST(joinRoute(urlPrefix, "/GetCommitHistory"), h.GetCommitHistory)
	engine.POST(joinRoute(urlPrefix, "/DiscardChanges"), h.DiscardChanges)
	engine.POST(joinRoute(urlPrefix, "/DeleteRepo"), h.DeleteRepo)
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

	// wedata-jupyter-server execution surface (proxied when gateway.url targets it)
	engine.GET("/api/kernels/execute_task/ws", h.ExecuteTaskWebSocket)
	engine.POST("/api/kernels/execute_task/save_outputs", h.SaveExecutionOutputs)
	engine.GET("/api/sessions/spark-app/stage", h.SparkAppStage)
	engine.GET("/api/spark-app/status", h.SparkAppStatus)
	engine.DELETE("/api/sessions/spark-app", h.DeleteSparkApp)
	engine.DELETE("/api/spark-sessions", h.DeleteSparkSessions)
	engine.GET("/api/sessions/python-packages", h.ListPythonPackages)
	engine.POST("/api/sessions/python-packages/requirements", h.WritePythonPackageRequirements)

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
