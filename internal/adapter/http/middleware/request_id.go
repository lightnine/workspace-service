package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(ctxmeta.HeaderRequestID))
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Header(ctxmeta.HeaderRequestID, requestID)
		c.Request = c.Request.WithContext(ctxmeta.WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}
