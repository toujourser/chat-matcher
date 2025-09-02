package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置允许的跨域请求来源
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // 允许所有域名，可以改为特定域名
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Trace-ID")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true") // 允许发送 Cookie
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")          // 预检请求缓存时间（秒）

		// 处理 OPTIONS 预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// 继续处理请求
		c.Next()
	}
}
