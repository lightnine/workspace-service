package middleware

import (
	"net/http"

	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		requestID, _ := ctxmeta.RequestIDFromContext(c.Request.Context())
		log.Error("panic recovered",
			zap.String("request_id", requestID),
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "internal server error",
		})
	})
}
