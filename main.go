package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/toujourser/chat-matcher/handler"
	"github.com/toujourser/chat-matcher/middlewares"
)

func main() {
	// 配置日志输出到文件
	setupLogger()

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
		api.POST("/match", handler.MatchHandle)
		api.GET("/ws", handler.WSHandle)
		api.GET("/chat/history", handler.ChatHistoryHandle)
		api.GET("/user/stats", handler.UserStatsHandle)
		api.GET("/user/rooms", handler.UserRoomsHandle)
	}

	// 静态文件服务
	r.Static("/static", "./static")
	log.Fatal(r.Run(port))
}

// setupLogger 配置日志输出到文件
func setupLogger() {
	// 创建 logs 目录
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
		return
	}

	// 生成日志文件名（按日期）
	now := time.Now()
	logFileName := filepath.Join(logsDir, "chat-matcher-"+now.Format("2006-01-02")+".log")

	// 打开或创建日志文件
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return
	}

	// 配置日志同时输出到控制台和文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("Logger initialized. Logs will be saved to: %s", logFileName)
}
