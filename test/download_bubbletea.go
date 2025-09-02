package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 1).
			Margin(1, 0)
)

// 消息类型
type progressMsg struct {
	downloaded int64
	total      int64
	speed      float64
	chunks     []*BubbleChunkProgress
}

type completedMsg struct {
	filename string
	fileSize int64
	duration time.Duration
	speed    float64
}

type errorMsg struct {
	err error
}

type statusMsg struct {
	message string
}

// Model 结构
type Model struct {
	progress    progress.Model
	downloaded  int64
	total       int64
	speed       float64
	chunks      []*BubbleChunkProgress
	status      string
	filename    string
	url         string
	threadCount int
	completed   bool
	err         error
	startTime   time.Time
	duration    time.Duration
	finalSpeed  float64
}

// 初始化模型
func NewModel(url, filename string, threadCount int) Model {
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 50

	return Model{
		progress:    p,
		filename:    filename,
		url:         url,
		threadCount: threadCount,
		status:      "准备开始下载...",
		startTime:   time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case progressMsg:
		m.downloaded = msg.downloaded
		m.total = msg.total
		m.speed = msg.speed
		m.chunks = msg.chunks

		percent := 0.0
		if m.total > 0 {
			percent = float64(m.downloaded) / float64(m.total)
		}

		cmd := m.progress.SetPercent(percent)
		return m, cmd

	case statusMsg:
		m.status = msg.message

	case completedMsg:
		m.completed = true
		m.duration = msg.duration
		m.finalSpeed = msg.speed
		m.status = "下载完成!"
		// 延迟2秒后自动退出，让用户能看到完成信息
		return m, tea.Sequence(
			tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return tea.Quit()
			}),
		)

	case errorMsg:
		m.err = msg.err
		m.status = "下载失败"

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return m.renderError()
	}

	if m.completed {
		return m.renderCompleted()
	}

	return m.renderProgress()
}

func (m Model) renderProgress() string {
	var b strings.Builder

	// 标题
	header := headerStyle.Render("🚀 多线程文件下载器")
	b.WriteString(header + "\n\n")

	// 文件信息
	fileInfo := fmt.Sprintf("📁 文件: %s\n🌐 URL: %s\n🧵 线程数: %d",
		m.filename, truncateURL(m.url, 60), m.threadCount)
	b.WriteString(infoStyle.Render(fileInfo) + "\n\n")

	// 状态信息
	b.WriteString(statusStyle.Render("📊 "+m.status) + "\n\n")

	// 总体进度
	progressInfo := fmt.Sprintf("总进度: %s / %s (%.1f%%) @ %.2f MB/s",
		bubbleFormatBytes(m.downloaded),
		bubbleFormatBytes(m.total),
		float64(m.downloaded)/float64(m.total)*100,
		m.speed)

	b.WriteString(subtitleStyle.Render(progressInfo) + "\n")
	b.WriteString(m.progress.View() + "\n\n")

	// 预计剩余时间
	if m.speed > 0 && m.total > m.downloaded {
		remaining := m.total - m.downloaded
		eta := time.Duration(float64(remaining)/m.speed/1024/1024) * time.Second
		etaInfo := fmt.Sprintf("⏱️  预计剩余时间: %v", eta.Truncate(time.Second))
		b.WriteString(subtitleStyle.Render(etaInfo) + "\n\n")
	}

	// 线程详情
	if len(m.chunks) > 0 {
		b.WriteString(titleStyle.Render("📄 线程详情:") + "\n")
		for _, chunk := range m.chunks {
			downloaded, total, speed, status := chunk.GetProgress()
			percent := 0.0
			if total > 0 {
				percent = float64(downloaded) / float64(total) * 100
			}

			statusIcon := getStatusIcon(status)
			chunkInfo := fmt.Sprintf("  %s 线程 %d: %.1f%% (%.1f KB/s) %s",
				statusIcon, chunk.Index, percent, speed, status)

			style := getStatusStyle(status)
			b.WriteString(style.Render(chunkInfo) + "\n")
		}
	}

	b.WriteString("\n" + subtitleStyle.Render("按 'q' 或 Ctrl+C 退出"))

	return boxStyle.Render(b.String())
}

func (m Model) renderCompleted() string {
	var b strings.Builder

	// 成功标题
	header := successStyle.Render("✅ 下载完成!")
	b.WriteString(header + "\n\n")

	// 完成信息
	info := fmt.Sprintf("📁 文件: %s\n📊 大小: %s\n⏱️  耗时: %v\n🚀 平均速度: %.2f MB/s\n🧵 线程数: %d",
		m.filename,
		bubbleFormatBytes(m.total),
		m.duration.Truncate(time.Millisecond),
		m.finalSpeed,
		m.threadCount)

	b.WriteString(infoStyle.Render(info) + "\n\n")
	b.WriteString(subtitleStyle.Render("🎉 程序将在 2 秒后自动退出..."))

	return boxStyle.Render(b.String())
}

func (m Model) renderError() string {
	var b strings.Builder

	// 错误标题
	header := errorStyle.Render("❌ 下载失败")
	b.WriteString(header + "\n\n")

	// 错误信息
	errorInfo := fmt.Sprintf("错误详情: %v", m.err)
	b.WriteString(errorStyle.Render(errorInfo) + "\n\n")
	b.WriteString(subtitleStyle.Render("按 'q' 或 Ctrl+C 退出"))

	return boxStyle.Render(b.String())
}

// 辅助函数
func getStatusIcon(status string) string {
	switch status {
	case "downloading":
		return "⬇️"
	case "completed":
		return "✅"
	case "error":
		return "❌"
	case "retrying":
		return "🔄"
	default:
		return "⏸️"
	}
}

func getStatusStyle(status string) lipgloss.Style {
	switch status {
	case "downloading":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // 蓝色
	case "completed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // 绿色
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // 红色
	case "retrying":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // 黄色
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // 灰色
	}
}

func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// BubbleDownloadChunk 表示一个下载分片
type BubbleDownloadChunk struct {
	Start int64
	End   int64
	Index int
}

// BubbleChunkProgress 分片进度
type BubbleChunkProgress struct {
	Index      int
	Total      int64
	Downloaded int64
	Speed      float64 // KB/s
	Status     string  // "downloading", "completed", "error", "retrying"
	LastUpdate time.Time
	mutex      sync.RWMutex
}

func (c *BubbleChunkProgress) Update(bytes int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	if !c.LastUpdate.IsZero() {
		duration := now.Sub(c.LastUpdate).Seconds()
		if duration > 0 {
			c.Speed = float64(bytes) / duration / 1024 // KB/s
		}
	}

	c.Downloaded += bytes
	c.LastUpdate = now
	if c.Status != "completed" && c.Status != "error" {
		c.Status = "downloading"
	}
}

func (c *BubbleChunkProgress) SetStatus(status string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Status = status
}

func (c *BubbleChunkProgress) GetProgress() (int64, int64, float64, string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Downloaded, c.Total, c.Speed, c.Status
}

// BubbleProgressTracker 进度跟踪器
type BubbleProgressTracker struct {
	Total      int64
	Downloaded int64
	Chunks     []*BubbleChunkProgress
	StartTime  time.Time
	mutex      sync.RWMutex
}

func (p *BubbleProgressTracker) Update(bytes int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Downloaded += bytes
}

func (p *BubbleProgressTracker) GetProgress() (int64, int64) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.Downloaded, p.Total
}

func (p *BubbleProgressTracker) GetOverallSpeed() float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.StartTime.IsZero() {
		return 0
	}

	duration := time.Since(p.StartTime).Seconds()
	if duration > 0 {
		return float64(p.Downloaded) / duration / 1024 / 1024 // MB/s
	}
	return 0
}

// BubbleMultiThreadDownloader 多线程下载器
type BubbleMultiThreadDownloader struct {
	URL         string
	Filename    string
	ThreadCount int
	ChunkSize   int64
	Progress    *BubbleProgressTracker
	Timeout     time.Duration
	MaxRetries  int
	program     *tea.Program
}

// NewBubbleMultiThreadDownloader 创建新的多线程下载器
func NewBubbleMultiThreadDownloader(url, filename string, threadCount int) *BubbleMultiThreadDownloader {
	return &BubbleMultiThreadDownloader{
		URL:         url,
		Filename:    filename,
		ThreadCount: threadCount,
		Progress:    &BubbleProgressTracker{StartTime: time.Now()},
		Timeout:     60 * time.Second, // 60秒超时
		MaxRetries:  3,                // 最大重试3次
	}
}

// 发送进度更新消息
func (d *BubbleMultiThreadDownloader) sendProgressUpdate() {
	if d.program != nil {
		downloaded, total := d.Progress.GetProgress()
		speed := d.Progress.GetOverallSpeed()

		d.program.Send(progressMsg{
			downloaded: downloaded,
			total:      total,
			speed:      speed,
			chunks:     d.Progress.Chunks,
		})
	}
}

// 发送状态消息
func (d *BubbleMultiThreadDownloader) sendStatus(message string) {
	if d.program != nil {
		d.program.Send(statusMsg{message: message})
	}
}

// 发送完成消息
func (d *BubbleMultiThreadDownloader) sendCompleted(duration time.Duration, speed float64) {
	if d.program != nil {
		d.program.Send(completedMsg{
			filename: d.Filename,
			fileSize: d.Progress.Total,
			duration: duration,
			speed:    speed,
		})
	}
}

// 发送错误消息
func (d *BubbleMultiThreadDownloader) sendError(err error) {
	if d.program != nil {
		d.program.Send(errorMsg{err: err})
	}
}

// getFileSize 获取远程文件大小
func (d *BubbleMultiThreadDownloader) getFileSize() (int64, bool, error) {
	client := &http.Client{
		Timeout: d.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	resp, err := client.Head(d.URL)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	// 检查是否支持断点续传
	supportsRange := resp.Header.Get("Accept-Ranges") == "bytes"

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, supportsRange, fmt.Errorf("无法获取文件大小")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, supportsRange, err
	}

	return size, supportsRange, nil
}

// startProgressMonitor 启动进度监控
func (d *BubbleMultiThreadDownloader) startProgressMonitor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms更新频率
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sendProgressUpdate()
		}
	}
}

// Download 执行多线程下载
func (d *BubbleMultiThreadDownloader) Download() error {
	d.sendStatus("正在获取文件信息...")

	// 获取文件信息
	fileSize, supportsRange, err := d.getFileSize()
	if err != nil {
		d.sendError(fmt.Errorf("获取文件信息失败: %v", err))
		return err
	}

	d.Progress.Total = fileSize
	d.sendStatus(fmt.Sprintf("文件大小: %s, 支持断点续传: %t", bubbleFormatBytes(fileSize), supportsRange))

	// 如果不支持断点续传或文件太小，使用单线程下载
	if !supportsRange || fileSize < int64(d.ThreadCount*1024*1024) {
		d.sendStatus("使用单线程下载...")
		return d.singleThreadDownload()
	}

	return d.multiThreadDownload(fileSize)
}

// multiThreadDownload 多线程下载
func (d *BubbleMultiThreadDownloader) multiThreadDownload(fileSize int64) error {
	// 创建临时目录
	tempDir := fmt.Sprintf("%s.tmp", d.Filename)
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		d.sendError(fmt.Errorf("创建临时目录失败: %v", err))
		return err
	}
	defer os.RemoveAll(tempDir)

	// 计算分片
	chunkSize := fileSize / int64(d.ThreadCount)
	var chunks []BubbleDownloadChunk

	// 初始化分片进度跟踪
	d.Progress.Chunks = make([]*BubbleChunkProgress, d.ThreadCount)

	for i := 0; i < d.ThreadCount; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1

		// 最后一个分片包含剩余的字节
		if i == d.ThreadCount-1 {
			end = fileSize - 1
		}

		chunk := BubbleDownloadChunk{
			Start: start,
			End:   end,
			Index: i,
		}
		chunks = append(chunks, chunk)

		// 初始化分片进度
		d.Progress.Chunks[i] = &BubbleChunkProgress{
			Index:  i,
			Total:  end - start + 1,
			Status: "waiting",
		}
	}

	d.sendStatus(fmt.Sprintf("使用 %d 线程下载，共 %d 个分片", d.ThreadCount, len(chunks)))

	// 启动进度监控
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.startProgressMonitor(ctx)

	// 启动下载协程
	var wg sync.WaitGroup
	errChan := make(chan error, d.ThreadCount)

	startTime := time.Now()

	for _, chunk := range chunks {
		wg.Add(1)
		go d.downloadChunk(chunk, tempDir, &wg, errChan)
	}

	// 等待所有下载完成
	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			d.sendError(err)
			return err
		}
	}

	// 合并文件
	d.sendStatus("正在合并文件...")
	err = d.mergeChunks(tempDir, len(chunks))
	if err != nil {
		d.sendError(err)
		return err
	}

	duration := time.Since(startTime)
	speed := float64(fileSize) / duration.Seconds() / 1024 / 1024 // MB/s

	d.sendCompleted(duration, speed)
	return nil
}

// singleThreadDownload 单线程下载
func (d *BubbleMultiThreadDownloader) singleThreadDownload() error {
	client := &http.Client{
		Timeout: d.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	resp, err := client.Get(d.URL)
	if err != nil {
		d.sendError(err)
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(d.Filename)
	if err != nil {
		d.sendError(err)
		return err
	}
	defer file.Close()

	// 启动进度监控
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.startProgressMonitor(ctx)

	startTime := time.Now()

	// 使用带缓冲的复制
	buffer := make([]byte, 64*1024) // 64KB缓冲区
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				d.sendError(writeErr)
				return writeErr
			}
			d.Progress.Update(int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			d.sendError(err)
			return err
		}
	}

	duration := time.Since(startTime)
	speed := float64(d.Progress.Total) / duration.Seconds() / 1024 / 1024

	d.sendCompleted(duration, speed)
	return nil
}

// downloadChunk 下载指定分片
func (d *BubbleMultiThreadDownloader) downloadChunk(chunk BubbleDownloadChunk, tempDir string, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	chunkProgress := d.Progress.Chunks[chunk.Index]
	chunkProgress.SetStatus("downloading")

	var err error
	for retry := 0; retry <= d.MaxRetries; retry++ {
		if retry > 0 {
			chunkProgress.SetStatus("retrying")
			time.Sleep(time.Duration(retry) * time.Second)
		}

		err = d.downloadChunkWithRetry(chunk, tempDir, chunkProgress)
		if err == nil {
			chunkProgress.SetStatus("completed")
			return
		}

		if retry == d.MaxRetries {
			chunkProgress.SetStatus("error")
			errChan <- fmt.Errorf("分片 %d 下载失败 (已重试 %d 次): %v", chunk.Index, d.MaxRetries, err)
			return
		}
	}
}

// downloadChunkWithRetry 单次下载尝试
func (d *BubbleMultiThreadDownloader) downloadChunkWithRetry(chunk BubbleDownloadChunk, tempDir string, chunkProgress *BubbleChunkProgress) error {
	tempFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", chunk.Index))
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer file.Close()

	req, err := http.NewRequest("GET", d.URL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End)
	req.Header.Set("Range", rangeHeader)

	client := &http.Client{
		Timeout: d.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("执行请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("返回错误状态码: %d", resp.StatusCode)
	}

	buffer := make([]byte, 64*1024)
	var downloaded int64 = 0
	expectedSize := chunk.End - chunk.Start + 1

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("写入文件失败: %v", writeErr)
			}

			chunkProgress.Update(int64(n))
			d.Progress.Update(int64(n))
			downloaded += int64(n)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取数据失败: %v", err)
		}

		if downloaded > expectedSize {
			return fmt.Errorf("下载数据量超出预期")
		}
	}

	if downloaded != expectedSize {
		return fmt.Errorf("下载数据量不匹配，期望: %d, 实际: %d", expectedSize, downloaded)
	}

	return nil
}

// mergeChunks 合并所有分片
func (d *BubbleMultiThreadDownloader) mergeChunks(tempDir string, chunkCount int) error {
	outputFile, err := os.Create(d.Filename)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer outputFile.Close()

	for i := 0; i < chunkCount; i++ {
		chunkFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", i))

		file, err := os.Open(chunkFile)
		if err != nil {
			return fmt.Errorf("打开分片文件 %d 失败: %v", i, err)
		}

		_, err = io.Copy(outputFile, file)
		file.Close()

		if err != nil {
			return fmt.Errorf("合并分片 %d 失败: %v", i, err)
		}

		os.Remove(chunkFile)
	}

	return nil
}

// bubbleFormatBytes 格式化字节数
func bubbleFormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 主函数
func main() {
	if len(os.Args) < 3 {
		fmt.Println("用法: go run download_bubbletea.go <URL> <输出文件名> [线程数]")
		fmt.Println("示例: go run download_bubbletea.go https://example.com/file.zip download.zip 8")
		return
	}

	url := os.Args[1]
	filename := os.Args[2]
	threadCount := 4 // 默认4线程

	if len(os.Args) > 3 {
		if count, err := strconv.Atoi(os.Args[3]); err == nil && count > 0 {
			threadCount = count
		}
	}

	// 创建下载器和模型
	downloader := NewBubbleMultiThreadDownloader(url, filename, threadCount)
	model := NewModel(url, filename, threadCount)

	// 创建 bubbletea 程序
	p := tea.NewProgram(model, tea.WithAltScreen())
	downloader.program = p

	// 在后台启动下载
	go func() {
		if err := downloader.Download(); err != nil {
			// 错误会通过消息传递到UI
		}
	}()

	// 启动UI
	if _, err := p.Run(); err != nil {
		fmt.Printf("运行程序时出错: %v\n", err)
		os.Exit(1)
	}
}
