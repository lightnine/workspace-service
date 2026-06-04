package response

import (
	"net/http"

	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

type Common struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type Body struct {
	Common    Common `json:"common"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Common: Common{
			Code: 0,
			Msg:  "ok",
		},
		Data:      data,
		RequestID: requestID(c),
	})
}

func Error(c *gin.Context, statusCode int, msg string) {
	c.JSON(statusCode, Body{
		Common: Common{
			Code: statusCode,
			Msg:  msg,
		},
		RequestID: requestID(c),
	})
}

func requestID(c *gin.Context) string {
	requestID, _ := ctxmeta.RequestIDFromContext(c.Request.Context())
	return requestID
}
