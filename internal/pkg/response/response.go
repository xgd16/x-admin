// Package response 统一 HTTP 响应格式。
//
// 所有接口都返回 {code, message, data, request_id} 信封，
// 分页接口的 data 为 {list, total, page, page_size}。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/requestid"
)

// Envelope 是统一响应信封。
type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// PageData 是分页接口的 data 结构。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// OK 返回 200 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestid.FromContext(c.Request.Context()),
	})
}

// Created 返回 201 创建成功响应。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestid.FromContext(c.Request.Context()),
	})
}

// NoContent 返回 204（无响应体）。
func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

// Page 返回统一的分页响应。
func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OK(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

// Error 写出标准化错误响应。
// 未知错误统一转为 500，并且不把内部错误信息泄露给客户端。
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
