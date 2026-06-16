package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"shop/internal/middleware"
)

func main() {
	r := gin.Default()

	r.Use(middleware.PrometheusMiddleware())

	r.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.Run(":8080")
}
