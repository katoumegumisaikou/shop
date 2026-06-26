package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HttpRequestTotal Http 各种方法请求总数
	HttpRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "shop-http-request-total",
		Help: "Http request total",
	}, []string{"method", "path", "status"})

	// HttpRequestDuration Http 各种方法的请求时间
	HttpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "shop-http-request-duration",
		Help:    "Http request duration second",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path"})

	// OrderCreatedTotal 下单总数
	OrderCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "shop-order-create-total",
		Help: "Shop order created total",
	})

	// PaymentTotal 支付总数
	PaymentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "shop-payment-total",
		Help: "payment total",
	})

	// SuccessPaymentTotal 成功交易总数
	SuccessPaymentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "shop-success-payment-total",
		Help: "success payment total",
	})
)

// PrometheusMiddleware promtheus 中间件，记录请求指标
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录接口完成的耗时
		timer := prometheus.NewTimer(HttpRequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
		))
		c.Next()
		timer.ObserveDuration()

		// 记录请求接口的详情
		HttpRequestTotal.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			fmt.Sprintf("%d", c.Writer.Status()),
		).Inc()
	}
}
