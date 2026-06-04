package router

import apphandler "git.woa.com/leondli/workspace-service/internal/adapter/http/handler"

// Handlers groups HTTP handlers wired in main. Fields are optional (nil = routes
// not registered). Add a new field here when introducing a feature; New's
// signature stays unchanged.
type Handlers struct {
	File    *apphandler.FileHandler
	Git     *apphandler.GitHandler
	Gateway *apphandler.GatewayHandler
}
