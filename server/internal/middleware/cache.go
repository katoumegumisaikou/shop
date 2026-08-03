package middleware

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"net/http"
	"shop/internal/pkg/errs"

	"github.com/gin-gonic/gin"
)

// PublicCache 为只读接口写 Cache-Control 响应头。
// maxAge: max-age 秒；swr: stale-while-revalidate 秒（0 则不加）。
func PublicCache(maxAge, swr int) gin.HandlerFunc {
	var cacheHeader string
	if maxAge <= 0 && swr <= 0 {
		cacheHeader = "no-cache"
	} else if swr > 0 {
		cacheHeader = fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d", maxAge, swr)
	} else {
		cacheHeader = fmt.Sprintf("public, max-age=%d", maxAge)
	}
	return func(c *gin.Context) {
		c.Header("Cache-Control", cacheHeader)
		c.Next()
	}
}

// 重写 ResponseWriter
type captureWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *captureWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

func (w *captureWriter) WriteHeader(code int) {
	w.status = code
}

// WriteHeaderNow 延迟到 flush，此处空操作。
func (w *captureWriter) WriteHeaderNow() {}

// Written 始终返回 false，阻止 gin 提前 flush。
func (w *captureWriter) Written() bool { return false }

func (w *captureWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *captureWriter) Size() int { return w.body.Len() }

// 优化原来的sha1采用速度更快的fnv
func (w *captureWriter) ETagCompute() string {
	h := fnv.New64a()
	h.Write(w.body.Bytes())
	return fmt.Sprintf(`"%x"`, h.Sum(nil))
}

func (w *captureWriter) flushCaptured(etagString string) (int, error) {
	if w.ResponseWriter == nil {
		return 0, errs.ErrInternal
	} else {
		w.ResponseWriter.Header().Set("ETag", etagString)
		w.ResponseWriter.WriteHeader(w.Status())
		return w.ResponseWriter.Write(w.body.Bytes())
	}
}

func ETagMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 不是 GET 方法的直接跳过
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// 调换原来的Response Writer
		newCW := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = newCW

		// 跳转到业务处理
		c.Next()

		etagString := newCW.ETagCompute()
		if match := c.GetHeader("If-None-Match"); match == etagString {
			newCW.ResponseWriter.WriteHeader(http.StatusNotModified)
			return
		}

		// http response
		_, _ = newCW.flushCaptured(etagString)

	}
}
