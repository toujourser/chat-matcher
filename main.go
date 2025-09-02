package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/toujourser/chat-matcher/handler"
	"github.com/toujourser/chat-matcher/middlewares"
)

func main() {
	// 初始化Redis连接
	redisConfig := handler.RedisConfig{
		Addr:     "localhost:6379", // 可以通过环境变量REDIS_ADDR覆盖
		Password: "",               // 可以通过环境变量REDIS_PASSWORD覆盖
		DB:       0,                // 使用默认数据库
	}

	redisManager := handler.NewRedisManager(redisConfig)
	defer redisManager.Close()

	// 创建Redis存储实例
	storage := handler.NewRedisStorage(redisManager)

	// 初始化处理器
	handler.InitializeHandlers(storage)

	port := ":9093"

	// 创建Gin引擎
	r := gin.Default()

	// 添加CORS中间件
	r.Use(middlewares.CORS())

	// 注册API路由
	api := r.Group("/api")
	{
		api.POST("/match", handler.GinHandleMatch)
		api.GET("/ws", handler.GinHandleWS)
		api.GET("/chat/history", handler.GinHandleChatHistory)
		api.GET("/user/stats", handler.GinHandleUserStats)
		api.GET("/user/rooms", handler.GinHandleUserRooms)
	}

	// 静态文件服务
	r.Static("/static", "./static")
	log.Fatal(r.Run(port))
}
