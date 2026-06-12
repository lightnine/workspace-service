package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apphandler "git.woa.com/leondli/workspace-service/internal/adapter/http/handler"
	httprouter "git.woa.com/leondli/workspace-service/internal/adapter/http/router"
	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	infrafs "git.woa.com/leondli/workspace-service/internal/infra/fs"
	infragateway "git.woa.com/leondli/workspace-service/internal/infra/gateway"
	infragit "git.woa.com/leondli/workspace-service/internal/infra/git"
	applogger "git.woa.com/leondli/workspace-service/internal/infra/logger"
	inframysql "git.woa.com/leondli/workspace-service/internal/infra/persistence/mysql"
	usecasefs "git.woa.com/leondli/workspace-service/internal/usecase/fs"
	usecasegit "git.woa.com/leondli/workspace-service/internal/usecase/git"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	configFile := flag.String("config", "conf/workspace-service.yaml", "path to yaml config file")
	flag.Parse()

	cfg, err := appconfig.Load(*configFile)
	if err != nil {
		panic(err)
	}

	log, err := applogger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = log.Sync()
	}()

	if cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	fileNodeStore, err := inframysql.NewOptionalFileNodeStore(cfg.MySQL)
	if err != nil {
		log.Fatal("init file node store failed", zap.Error(err))
	}
	if fileNodeStore == nil {
		log.Warn("mysql dsn is empty, ws_file_node recording is disabled")
	}

	mountRoot := usecasegit.CleanMountRoot(cfg.Workspace.MountRoot)
	workspaceFSClient := infrafs.NewWorkspaceFSClient(fileNodeStore, mountRoot)
	gitMetaRoot := usecasegit.CleanMountRoot(cfg.Workspace.GitMetaRoot)
	workspaceGitClient := infragit.NewWorkspaceGitClient(fileNodeStore, mountRoot, gitMetaRoot)
	gitService := usecasegit.NewService(workspaceGitClient, mountRoot, fileNodeStore)
	fileService := usecasefs.NewService(workspaceFSClient, mountRoot, gitService)
	fileHandler := apphandler.NewFileHandler(fileService)
	if gitMetaRoot != "" {
		log.Info("git metadata stored outside workspace mount", zap.String("git_meta_root", gitMetaRoot))
	}
	gitHandler := apphandler.NewGitHandler(gitService)

	sessionStore, err := inframysql.NewOptionalKernelSessionStore(cfg.MySQL)
	if err != nil {
		log.Fatal("init kernel session store failed", zap.Error(err))
	}
	if sessionStore == nil {
		log.Warn("mysql dsn is empty, ws_kernel_session recording is disabled")
	}

	gatewayClient, err := infragateway.NewOptionalGateway(cfg.Gateway, log)
	if err != nil {
		log.Fatal("init gateway client failed", zap.Error(err))
	}
	var gatewayHandler *apphandler.GatewayHandler
	if gatewayClient == nil {
		log.Warn("gateway url is empty, jupyter gateway proxy is disabled")
	} else {
		gatewayClient = infragateway.WrapWithSessionStore(gatewayClient, sessionStore, log)
		gatewayHandler = apphandler.NewGatewayHandler(gatewayClient)
	}

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler: httprouter.NewWithHandlers(log, cfg.Server.URLPrefix, &httprouter.Handlers{
			File:    fileHandler,
			Git:     gitHandler,
			Gateway: gatewayHandler,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("starting workspace service",
			zap.String("name", cfg.Server.Name),
			zap.String("address", cfg.Server.Address),
			zap.String("url_prefix", cfg.Server.URLPrefix),
			zap.String("workspace_mount_root", cfg.Workspace.MountRoot),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil {
			log.Fatal("server stopped unexpectedly", zap.Error(err))
		}
		return
	}

	timeout := cfg.Server.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("server shutdown failed", zap.Error(err))
	}

	log.Info("workspace service stopped")
}
