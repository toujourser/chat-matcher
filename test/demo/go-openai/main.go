package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

func main() {
	// 从环境变量获取 API Key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置 OPENAI_API_KEY 环境变量")
		return
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://tbai.xin/v1"

	// 创建客户端
	client := openai.NewClientWithConfig(config)

	// 演示不同的功能
	fmt.Println("=== OpenAI Go SDK Demo ===\n")

	// 1. 简单的聊天完成
	fmt.Println("1. 简单聊天:")
	simpleChat(client)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 2. 流式聊天
	fmt.Println("2. 流式聊天:")
	streamChat(client)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 3. 系统提示词 + 多轮对话
	fmt.Println("3. 系统提示词 + 多轮对话:")
	multiTurnChat(client)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 4. 图像生成 (DALL-E)
	fmt.Println("4. 图像生成:")
	generateImage(client)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 5. 文本 Embedding
	fmt.Println("5. 文本 Embedding:")
	createEmbedding(client)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 6. 自定义配置示例
	customConfigExample()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

}

// 简单聊天完成
func simpleChat(client *openai.Client) {
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "请用一句话介绍 Go 语言的特点",
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("聊天完成错误: %v\n", err)
		return
	}

	log.Printf("回答: %s\n", resp.Choices[0].Message.Content)
	log.Printf("使用 tokens: %d\n", resp.Usage.TotalTokens)
}

// 流式聊天
func streamChat(client *openai.Client) {
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "请写一个关于 Go 语言并发的简短说明",
			},
		},
		Stream: true,
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		log.Printf("创建流式聊天错误: %v\n", err)
		return
	}
	defer stream.Close()

	fmt.Print("回答: ")
	for {
		response, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("流式接收错误: %v\n", err)
			return
		}

		if len(response.Choices) > 0 {
			fmt.Print(response.Choices[0].Delta.Content)
		}
	}
	fmt.Println()
}

// 系统提示词 + 多轮对话
func multiTurnChat(client *openai.Client) {
	ctx := context.Background()

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "你是一个专业的 Go 语言专家，请提供简洁准确的技术回答。",
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "什么是 goroutine？",
		},
	}

	// 第一次对话
	req := openai.ChatCompletionRequest{
		Model:     openai.GPT4oMini,
		Messages:  messages,
		MaxTokens: 150,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("多轮聊天错误: %v\n", err)
		return
	}

	// 添加 AI 的回答到对话历史
	messages = append(messages, resp.Choices[0].Message)
	log.Printf("AI: %s\n\n", resp.Choices[0].Message.Content)

	// 继续对话
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "能给个简单的代码示例吗？",
	})

	req.Messages = messages
	resp, err = client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("第二轮对话错误: %v\n", err)
		return
	}

	log.Printf("AI: %s\n", resp.Choices[0].Message.Content)
}

// 图像生成
func generateImage(client *openai.Client) {
	ctx := context.Background()

	req := openai.ImageRequest{
		Prompt: "A cute gopher (Go language mascot) coding on a computer, digital art style",
		N:      1,
		Size:   openai.CreateImageSize256x256,
	}

	resp, err := client.CreateImage(ctx, req)
	if err != nil {
		log.Printf("图像生成错误: %v\n", err)
		return
	}

	if len(resp.Data) > 0 {
		log.Printf("图像 URL: %s\n", resp.Data[0].URL)
	}
}

// 创建文本 Embedding
func createEmbedding(client *openai.Client) {
	ctx := context.Background()

	req := openai.EmbeddingRequest{
		Input: []string{
			"Go 是 Google 开发的开源编程语言",
			"Go 语言具有简洁的语法和强大的并发特性",
		},
		Model: openai.AdaEmbeddingV2,
	}

	resp, err := client.CreateEmbeddings(ctx, req)
	if err != nil {
		log.Printf("创建 Embedding 错误: %v\n", err)
		return
	}

	for i, embedding := range resp.Data {
		log.Printf("文本 %d 的 Embedding 维度: %d\n", i+1, len(embedding.Embedding))
		log.Printf("前5个维度值: %.6f, %.6f, %.6f, %.6f, %.6f\n",
			embedding.Embedding[0],
			embedding.Embedding[1],
			embedding.Embedding[2],
			embedding.Embedding[3],
			embedding.Embedding[4])
	}
}

// 自定义配置示例（用于兼容其他 OpenAI 规范的 API）
func customConfigExample() {
	// 自定义配置，可以用于其他兼容 OpenAI 规范的 API
	config := openai.DefaultConfig("your-api-key")

	// 设置自定义 API 端点
	config.BaseURL = "https://api.your-service.com/v1"

	// 设置自定义 Headers（如果需要）
	// config.HTTPClient = &http.Client{
	//     Timeout: 30 * time.Second,
	// }

	client := openai.NewClientWithConfig(config)

	// 然后正常使用 client...
	_ = client
}
