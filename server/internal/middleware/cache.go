package middleware

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"net/http"
	"shop/internal/pkg/errs"

	"github.com/gin-gonic/gin"
)

// PublicCache 为只读接口设置 Cache-Control 响应头，控制浏览器/CDN 缓存行为。
//
// 参数：
//
//	maxAge —— 缓存有效时长（秒），超过后缓存过期需重新验证
//	swr   —— stale-while-revalidate 时长（秒），过期后仍可用旧缓存，同时后台刷新
//
// 示例：
//
//	PublicCache(60, 0)   → Cache-Control: public, max-age=60           // 严格 60 秒
//	PublicCache(60, 30)  → Cache-Control: public, max-age=60, stale-while-revalidate=30
//	PublicCache(0, 0)    → Cache-Control: no-cache                     // 每次必须验证
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

// ETagMiddleware 实现基于 ETag 的 HTTP 条件请求校验，减少不必要的数据传输。
//
// 工作流程：
//  1. 拦截 handler 的响应，计算 body 的哈希作为 ETag
//  2. 与请求头 If-None-Match 对比：
//     匹配 → 返回 304 Not Modified（body 为空，省流量）
//     不匹配 → 正常返回 body + ETag 响应头
//
// 仅处理 GET 请求，非 GET 直接放行。
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
