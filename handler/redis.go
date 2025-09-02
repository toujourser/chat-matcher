package handler

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RedisManager Redis管理器
type RedisManager struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisManager 创建Redis管理器
func NewRedisManager(config RedisConfig) *RedisManager {
	// 如果没有配置地址，使用默认值
	if config.Addr == "" {
		config.Addr = "localhost:6379"
	}

	// 支持从环境变量读取配置
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		config.Addr = addr
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Password = password
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx := context.Background()

	// 测试连接
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		// 在开发环境中，如果Redis连接失败，程序仍然可以运行（只是不会存储数据）
		// 在生产环境中，可以选择直接退出
	} else {
		log.Println("Successfully connected to Redis")
	}

	return &RedisManager{
		client: rdb,
		ctx:    ctx,
	}
}

// GetClient 获取Redis客户端
func (rm *RedisManager) GetClient() *redis.Client {
	return rm.client
}

// GetContext 获取上下文
func (rm *RedisManager) GetContext() context.Context {
	return rm.ctx
}

// Close 关闭Redis连接
func (rm *RedisManager) Close() error {
	return rm.client.Close()
}

// IsConnected 检查Redis连接状态
func (rm *RedisManager) IsConnected() bool {
	_, err := rm.client.Ping(rm.ctx).Result()
	return err == nil
}
