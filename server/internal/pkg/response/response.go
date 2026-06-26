// Package response provides unified HTTP JSON responses for Gin handlers.
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
)

// Body is the standard API response body.
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK writes a successful JSON response.
func OK(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Body{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Error writes a unified error JSON response and aborts the request chain.
func Error(ctx *gin.Context, err error) {
	if err == nil {
		return
	}

	var appErr *errs.AppError
	if !errors.As(err, &appErr) {
		appErr = errs.ErrInternal
	}

	ctx.AbortWithStatusJSON(appErr.HTTPStatus, Body{
		Code:    appErr.Code,
		Message: appErr.Message,
	})
}
