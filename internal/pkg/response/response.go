package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/requestid"
)

type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestid.FromContext(c.Request.Context()),
	})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestid.FromContext(c.Request.Context()),
	})
}

func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OK(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

// Error writes a normalized error envelope. Unknown errors become 500 and never
// leak their internal message.
func Error(c *gin.Context, err error) {
	biz := apperr.From(err)
	message := biz.Message
	if biz.Code == apperr.CodeInternal {
		message = "服务器内部错误"
	}
	if biz.Code == apperr.CodeOK {
		biz.Code = apperr.CodeInternal
	}
	c.AbortWithStatusJSON(apperr.HTTPStatus(biz.Code), Envelope{
		Code:      int(biz.Code),
		Message:   message,
		RequestID: requestid.FromContext(c.Request.Context()),
	})
}
