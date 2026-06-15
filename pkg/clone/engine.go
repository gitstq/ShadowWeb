// Package clone 提供网站爬取引擎，采用双层 Worker 池架构：
// - 渲染 Worker 池：使用 Headless Chrome 渲染动态页面
// - 资源 Worker 池：下载 CSS、图片、字体等静态资源
package clone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
	"github.com/demyanovs/robotstxt"
)

// State 保存爬取状态，用于断点续爬
type State struct {
	Version      string            `json:"version"`
	TargetURL    string            `json:"target_url"`
	Visited      map[string]bool   `json:"visited"`
	Queue        []string          `json:"queue"`
	Downloaded   map[string]string `json:"downloaded"` // URL -> 本地路径
	LastModified time.Time         `json:"last_modified"`
}

// Config 定义爬取引擎配置
type Config struct {
	TargetURL     string
	OutputDir     string
	ScopePrefix   string
	ExcludeList   []string
	RateLimit     float64
	CookieFile    string
	NoJS          bool
	RespectRobots bool
	MaxDepth      int
	Resume        bool
	Concurrency   int
	UserAgent     string
}

// Engine 是爬取引擎的核心结构
type Engine struct {
	config     Config
	state      State
	statePath  string
	robots     *robotstxt.RobotsData
	client     *http.Client

	// 计数器 (Go 1.18 兼容：使用 uint64 通过 atomic 包函数操作)
	visitedCount   uint64
	downloadCount  uint64
	errorCount     uint64

	// 同步原语
	mu         sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc

	// URL 队列
	queue      chan *Task
	renderPool chan struct{} // 渲染 Worker 池令牌
	assetPool  chan struct{} // 资源 Worker 池令牌

	// 去重集合
	seen       map[string]bool
	seenMu     sync.RWMutex
}

// Task 表示一个爬取任务
type Task struct {
	URL       string
	Depth     int
	TaskType  TaskType
	Referer   string
}

// TaskType 定义任务类型
type TaskType int

const (
	TaskPage   TaskType = iota // HTML 页面渲染任务
	TaskAsset                  // 静态资源下载任务
)

// DefaultUserAgent 是默认的 User-Agent 字符串
const DefaultUserAgent = "ShadowWeb/1.0 (Offline Archiver; +https://github.com/gitstq/ShadowWeb)"

// NewEngine 创建新的爬取引擎实例
func NewEngine(cfg Config) (*Engine, error) {
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	ctx, cancel := context.WithCancel(context.Background())

	eng := &Engine{
		config:     cfg,
		statePath:  filepath.Join(cfg.OutputDir, "state.json"),
		client:     &http.Client{Timeout: 30 * time.Second},
		ctx:        ctx,
		cancel:     cancel,
		queue:      make(chan *Task, 1000),
		renderPool: make(chan struct{}, cfg.Concurrency),
		assetPool:  make(chan struct{}, cfg.Concurrency*2),
		seen:       make(map[string]bool),
	}

	// 初始化渲染和资源 Worker 池令牌
	for i := 0; i < cfg.Concurrency; i++ {
		eng.renderPool <- struct{}{}
	}
	for i := 0; i < cfg.Concurrency*2; i++ {
		eng.assetPool <- struct{}{}
	}

	// 尝试加载已有状态
	if cfg.Resume {
		if err := eng.loadState(); err != nil {
			return nil, fmt.Errorf("加载状态文件失败: %w", err)
		}
	} else {
		eng.state = State{
			Version:    "1.0",
			TargetURL:  cfg.TargetURL,
			Visited:    make(map[string]bool),
			Downloaded: make(map[string]string),
		}
	}

	// 加载 robots.txt
	if cfg.RespectRobots {
		if err := eng.loadRobots(); err != nil && verboseEnabled() {
			fmt.Fprintf(os.Stderr, "[警告] 加载 robots.txt 失败: %v\n", err)
		}
	}

	return eng, nil
}

// Run 启动爬取引擎
func (e *Engine) Run() error {
	defer e.cancel()

	// 确保输出目录存在
	if err := os.MkdirAll(e.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 启动状态保存协程
	go e.stateSaver()

	// 将起始 URL 加入队列
	startURL := e.config.TargetURL
	if !e.isSeen(startURL) {
		e.markSeen(startURL)
		e.queue <- &Task{URL: startURL, Depth: 0, TaskType: TaskPage}
	}

	// 启动 Worker
	var workerWg sync.WaitGroup
	for i := 0; i < e.config.Concurrency; i++ {
		workerWg.Add(1)
		go e.renderWorker(&workerWg)
	}
	for i := 0; i < e.config.Concurrency*2; i++ {
		workerWg.Add(1)
		go e.assetWorker(&workerWg)
	}

	// 等待队列处理完成
	// 使用一个监控协程来检测何时队列为空且没有活跃 Worker
	done := make(chan struct{})
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if len(e.queue) == 0 && e.wgCounter() == 0 {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		fmt.Println("[ShadowWeb] 爬取任务完成")
	case <-e.ctx.Done():
		fmt.Println("[ShadowWeb] 爬取任务被取消")
	}

	// 关闭队列，等待 Worker 退出
	close(e.queue)
	workerWg.Wait()

	// 保存最终状态
	return e.saveState()
}

// Stop 停止爬取引擎
func (e *Engine) Stop() {
	e.cancel()
}

// renderWorker 渲染 Worker，处理 HTML 页面
func (e *Engine) renderWorker(wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range e.queue {
		if task.TaskType != TaskPage {
			// 非页面任务重新放回队列（由资源 Worker 处理）
			select {
			case e.queue <- task:
			default:
			}
			continue
		}

		if e.isVisited(task.URL) {
			continue
		}

		// 获取渲染 Worker 令牌
		select {
		case <-e.renderPool:
		case <-e.ctx.Done():
			return
		}

		e.wg.Add(1)
		go func(t *Task) {
			defer e.wg.Done()
			defer func() { e.renderPool <- struct{}{} }()

			if err := e.processPage(t); err != nil {
				atomic.AddUint64(&e.errorCount, 1)
				if verboseEnabled() {
					fmt.Fprintf(os.Stderr, "[错误] 处理页面失败 %s: %v\n", t.URL, err)
				}
			}
		}(task)
	}
}

// assetWorker 资源 Worker，处理静态资源下载
func (e *Engine) assetWorker(wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range e.queue {
		if task.TaskType != TaskAsset {
			// 非资源任务重新放回队列
			select {
			case e.queue <- task:
			default:
			}
			continue
		}

		if e.isDownloaded(task.URL) {
			continue
		}

		// 获取资源 Worker 令牌
		select {
		case <-e.assetPool:
		case <-e.ctx.Done():
			return
		}

		e.wg.Add(1)
		go func(t *Task) {
			defer e.wg.Done()
			defer func() { e.assetPool <- struct{}{} }()

			if err := e.processAsset(t); err != nil {
				atomic.AddUint64(&e.errorCount, 1)
				if verboseEnabled() {
					fmt.Fprintf(os.Stderr, "[错误] 下载资源失败 %s: %v\n", t.URL, err)
				}
			}
		}(task)
	}
}

// processPage 处理 HTML 页面
func (e *Engine) processPage(task *Task) error {
	if task.Depth > e.config.MaxDepth {
		return nil
	}

	if !e.canFetch(task.URL) {
		if verboseEnabled() {
			fmt.Fprintf(os.Stderr, "[跳过] robots.txt 禁止访问: %s\n", task.URL)
		}
		return nil
	}

	// 限速控制
	if e.config.RateLimit > 0 {
		time.Sleep(time.Duration(1.0/e.config.RateLimit*1000) * time.Millisecond)
	}

	// 下载页面内容
	body, err := e.fetchURL(task.URL)
	if err != nil {
		return fmt.Errorf("下载页面失败: %w", err)
	}
	defer body.Close()

	// 解析 HTML
	doc, err := html.Parse(body)
	if err != nil {
		return fmt.Errorf("解析 HTML 失败: %w", err)
	}

	// 提取页面中的链接和资源
	links, assets := e.extractLinksAndAssets(doc, task.URL)

	// 保存 HTML 文件
	if err := e.saveHTML(task.URL, doc); err != nil {
		return fmt.Errorf("保存 HTML 失败: %w", err)
	}

	// 标记为已访问
	e.markVisited(task.URL)
	atomic.AddUint64(&e.visitedCount, 1)

	// 将发现的资源加入队列
	for _, assetURL := range assets {
		if !e.isSeen(assetURL) && e.isInScope(assetURL) {
			e.markSeen(assetURL)
			select {
			case e.queue <- &Task{URL: assetURL, Depth: task.Depth, TaskType: TaskAsset, Referer: task.URL}:
			case <-e.ctx.Done():
				return e.ctx.Err()
			}
		}
	}

	// 将发现的页面链接加入队列
	for _, link := range links {
		if !e.isSeen(link) && e.isInScope(link) {
			e.markSeen(link)
			select {
			case e.queue <- &Task{URL: link, Depth: task.Depth + 1, TaskType: TaskPage, Referer: task.URL}:
			case <-e.ctx.Done():
				return e.ctx.Err()
			}
		}
	}

	if verboseEnabled() {
		fmt.Fprintf(os.Stderr, "[页面] 已处理 (%d/%d 错误) %s\n", atomic.LoadUint64(&e.visitedCount), atomic.LoadUint64(&e.errorCount), task.URL)
	}

	return nil
}

// processAsset 处理静态资源下载
func (e *Engine) processAsset(task *Task) error {
	if !e.canFetch(task.URL) {
		return nil
	}

	// 限速控制
	if e.config.RateLimit > 0 {
		time.Sleep(time.Duration(1.0/e.config.RateLimit*1000) * time.Millisecond)
	}

	body, err := e.fetchURL(task.URL)
	if err != nil {
		return fmt.Errorf("下载资源失败: %w", err)
	}
	defer body.Close()

	// 保存资源文件
	localPath, err := e.saveAsset(task.URL, body)
	if err != nil {
		return fmt.Errorf("保存资源失败: %w", err)
	}

	e.markDownloaded(task.URL, localPath)
	atomic.AddUint64(&e.downloadCount, 1)

	if verboseEnabled() {
		fmt.Fprintf(os.Stderr, "[资源] 已下载 (%d) %s -> %s\n", atomic.LoadUint64(&e.downloadCount), task.URL, localPath)
	}

	return nil
}

// fetchURL 发送 HTTP GET 请求获取内容
func (e *Engine) fetchURL(targetURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(e.ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", e.config.UserAgent)
	if task, ok := e.ctx.Value("task").(*Task); ok && task.Referer != "" {
		req.Header.Set("Referer", task.Referer)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// extractLinksAndAssets 从 HTML 文档中提取链接和资源 URL
func (e *Engine) extractLinksAndAssets(doc *html.Node, baseURL string) (links []string, assets []string) {
	base, _ := url.Parse(baseURL)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						if resolved := resolveURL(base, attr.Val); resolved != "" {
							links = append(links, resolved)
						}
					}
				}
			case "img", "script", "link", "source", "video", "audio", "iframe":
				for _, attr := range n.Attr {
					if attr.Key == "src" || attr.Key == "href" || attr.Key == "data-src" {
						if resolved := resolveURL(base, attr.Val); resolved != "" {
							assets = append(assets, resolved)
						}
					}
				}
			case "style":
				// 内联样式中可能有 background-image 等
				if n.FirstChild != nil {
					styleAssets := extractURLsFromCSS(n.FirstChild.Data)
					for _, u := range styleAssets {
						if resolved := resolveURL(base, u); resolved != "" {
							assets = append(assets, resolved)
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return links, assets
}

// saveHTML 保存处理后的 HTML 文档
func (e *Engine) saveHTML(targetURL string, doc *html.Node) error {
	localPath := e.urlToLocalPath(targetURL, ".html")
	fullPath := filepath.Join(e.config.OutputDir, localPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return html.Render(file, doc)
}

// saveAsset 保存静态资源
func (e *Engine) saveAsset(targetURL string, body io.Reader) (string, error) {
	localPath := e.urlToLocalPath(targetURL, "")
	fullPath := filepath.Join(e.config.OutputDir, localPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, body)
	if err != nil {
		return "", err
	}

	return localPath, nil
}

// urlToLocalPath 将 URL 转换为本地文件路径
func (e *Engine) urlToLocalPath(targetURL string, defaultExt string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return ""
	}

	path := u.Path
	if path == "" || strings.HasSuffix(path, "/") {
		path += "index" + defaultExt
	}
	if filepath.Ext(path) == "" && defaultExt != "" {
		path += defaultExt
	}

	// 清理路径，防止目录遍历
	path = filepath.Clean(path)
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	return path
}

// resolveURL 解析相对 URL 为绝对 URL
func resolveURL(base *url.URL, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") || strings.HasPrefix(ref, "mailto:") {
		return ""
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(refURL)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}

	return resolved.String()
}

// cssURLRegex 用于从 CSS 文本中提取 URL
var cssURLRegex = regexp.MustCompile(`url\(["']?([^"')]+)["']?\)`)

// extractURLsFromCSS 从 CSS 文本中提取 URL
func extractURLsFromCSS(css string) []string {
	var urls []string
	matches := cssURLRegex.FindAllStringSubmatch(css, -1)
	for _, m := range matches {
		if len(m) > 1 {
			urls = append(urls, m[1])
		}
	}
	return urls
}

// isInScope 检查 URL 是否在作用域内
func (e *Engine) isInScope(targetURL string) bool {
	// 检查排除列表
	for _, pattern := range e.config.ExcludeList {
		if strings.Contains(targetURL, pattern) {
			return false
		}
	}

	// 检查作用域前缀
	if e.config.ScopePrefix != "" {
		if !strings.HasPrefix(targetURL, e.config.ScopePrefix) {
			return false
		}
	}

	return true
}

// canFetch 检查是否允许爬取（robots.txt）
func (e *Engine) canFetch(targetURL string) bool {
	if e.robots == nil || !e.config.RespectRobots {
		return true
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	return e.robots.IsAllowed(e.config.UserAgent, u.Path)
}

// loadRobots 加载目标网站的 robots.txt
func (e *Engine) loadRobots() error {
	u, err := url.Parse(e.config.TargetURL)
	if err != nil {
		return err
	}

	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	resp, err := e.client.Get(robotsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("robots.txt 返回 HTTP %d", resp.StatusCode)
	}

	var robotsErr error
	e.robots, robotsErr = robotstxt.FromResponse(resp)
	return robotsErr
}

// stateSaver 定期保存状态
func (e *Engine) stateSaver() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := e.saveState(); err != nil && verboseEnabled() {
				fmt.Fprintf(os.Stderr, "[警告] 保存状态失败: %v\n", err)
			}
		case <-e.ctx.Done():
			return
		}
	}
}

// saveState 保存爬取状态到文件
func (e *Engine) saveState() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.LastModified = time.Now()

	data, err := json.MarshalIndent(e.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(e.statePath, data, 0644)
}

// loadState 从文件加载爬取状态
func (e *Engine) loadState() error {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &e.state)
}

// markVisited 标记 URL 为已访问
func (e *Engine) markVisited(targetURL string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Visited[targetURL] = true
}

// isVisited 检查 URL 是否已访问
func (e *Engine) isVisited(targetURL string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Visited[targetURL]
}

// markDownloaded 标记资源为已下载
func (e *Engine) markDownloaded(targetURL, localPath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Downloaded[targetURL] = localPath
}

// isDownloaded 检查资源是否已下载
func (e *Engine) isDownloaded(targetURL string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.state.Downloaded[targetURL]
	return ok
}

// markSeen 标记 URL 已加入队列
func (e *Engine) markSeen(targetURL string) {
	e.seenMu.Lock()
	defer e.seenMu.Unlock()
	e.seen[targetURL] = true
}

// isSeen 检查 URL 是否已加入队列
func (e *Engine) isSeen(targetURL string) bool {
	e.seenMu.RLock()
	defer e.seenMu.RUnlock()
	return e.seen[targetURL]
}

// wgCounter 返回当前活跃任务数（近似值）
func (e *Engine) wgCounter() int {
	// 由于 sync.WaitGroup 没有提供读取计数的方法，
	// 这里使用一个简化方案：检查队列长度和已处理数量
	return len(e.queue)
}

// Stats 返回当前统计信息
func (e *Engine) Stats() (visited, downloaded, errors uint64) {
	return atomic.LoadUint64(&e.visitedCount), atomic.LoadUint64(&e.downloadCount), atomic.LoadUint64(&e.errorCount)
}

// verboseEnabled 检查是否启用详细输出
func verboseEnabled() bool {
	// 通过环境变量或全局变量控制
	return os.Getenv("SHADOWWEB_VERBOSE") == "1"
}
