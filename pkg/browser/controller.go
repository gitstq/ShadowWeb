// Package browser 提供 Headless Chrome 控制功能，基于 go-rod 库实现。
// 负责页面渲染、JavaScript 执行、截图和 DOM 操作。
package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Controller 是浏览器控制器，管理 Headless Chrome 实例
type Controller struct {
	browser     *rod.Browser
	launcher    *launcher.Launcher
	userAgent   string
	cookieFile  string

	// 页面池，复用页面实例
	pagePool    chan *rod.Page
	poolSize    int

	// 同步
	mu          sync.Mutex
	closed      bool
}

// Config 定义浏览器控制器配置
type Config struct {
	Headless    bool   // 是否无头模式
	UserAgent   string // User-Agent
	CookieFile  string // Cookie 文件路径
	WindowWidth int    // 窗口宽度
	WindowHeight int   // 窗口高度
	PoolSize    int    // 页面池大小
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Headless:     true,
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		WindowWidth:  1920,
		WindowHeight: 1080,
		PoolSize:     4,
	}
}

// NewController 创建新的浏览器控制器
func NewController(cfg Config) (*Controller, error) {
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 4
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultConfig().UserAgent
	}

	// 启动 Chrome
	launch := launcher.New().
		Headless(cfg.Headless).
		NoSandbox(true).
		Set("disable-gpu", "true").
		Set("disable-dev-shm-usage", "true").
		Set("disable-setuid-sandbox", "true").
		Set("disable-web-security", "true").
		Set("disable-features", "IsolateOrigins,site-per-process").
		Set("disable-blink-features", "AutomationControlled")

	// 尝试查找系统 Chrome
	if path, exists := launcher.LookPath(); exists {
		launch = launch.Bin(path)
	}

	u, err := launch.Launch()
	if err != nil {
		return nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("连接 Chrome 失败: %w", err)
	}

	ctrl := &Controller{
		browser:    browser,
		launcher:   launch,
		userAgent:  cfg.UserAgent,
		cookieFile: cfg.CookieFile,
		pagePool:   make(chan *rod.Page, cfg.PoolSize),
		poolSize:   cfg.PoolSize,
	}

	// 初始化页面池
	for i := 0; i < cfg.PoolSize; i++ {
		page, err := ctrl.newPage(cfg.WindowWidth, cfg.WindowHeight)
		if err != nil {
			ctrl.Close()
			return nil, fmt.Errorf("创建页面失败: %w", err)
		}
		ctrl.pagePool <- page
	}

	// 加载 Cookie
	if cfg.CookieFile != "" {
		if err := ctrl.loadCookies(cfg.CookieFile); err != nil {
			fmt.Fprintf(os.Stderr, "[警告] 加载 Cookie 失败: %v\n", err)
		}
	}

	return ctrl, nil
}

// newPage 创建新页面并设置参数
func (c *Controller) newPage(width, height int) (*rod.Page, error) {
	page, err := c.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, err
	}

	// 设置视口
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  width,
		Height: height,
	}); err != nil {
		page.Close()
		return nil, err
	}

	// 设置 User-Agent
	if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: c.userAgent,
	}); err != nil {
		page.Close()
		return nil, err
	}

	// 注入脚本隐藏 webdriver 特征
	hideScript := `
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});
		Object.defineProperty(navigator, 'plugins', {
			get: () => [1, 2, 3, 4, 5]
		});
		window.chrome = { runtime: {} };
	`
	if _, err := page.EvalOnNewDocument(hideScript); err != nil {
		page.Close()
		return nil, err
	}

	return page, nil
}

// RenderPage 渲染指定 URL 并返回 HTML 内容
func (c *Controller) RenderPage(ctx context.Context, targetURL string) (string, error) {
	page, err := c.acquirePage()
	if err != nil {
		return "", err
	}
	defer c.releasePage(page)

	// 导航到目标页面
	err = page.Context(ctx).Navigate(targetURL)
	if err != nil {
		return "", fmt.Errorf("导航到 %s 失败: %w", targetURL, err)
	}

	// 等待页面加载完成
	if err := page.Context(ctx).WaitLoad(); err != nil {
		return "", fmt.Errorf("等待页面加载失败: %w", err)
	}

	// 等待网络空闲（最多 5 秒）
	page.Context(ctx).WaitIdle(5 * time.Second)

	// 获取 HTML 内容
	html, err := page.HTML()
	if err != nil {
		return "", fmt.Errorf("获取页面 HTML 失败: %w", err)
	}

	return html, nil
}

// RenderPageWithAssets 渲染页面并提取所有资源 URL
func (c *Controller) RenderPageWithAssets(ctx context.Context, targetURL string) (html string, assets []string, err error) {
	page, err := c.acquirePage()
	if err != nil {
		return "", nil, err
	}
	defer c.releasePage(page)

	// 拦截请求以收集资源 URL
	var assetURLs []string
	var assetMu sync.Mutex

	go page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		if e.Request.URL != targetURL {
			assetMu.Lock()
			assetURLs = append(assetURLs, e.Request.URL)
			assetMu.Unlock()
		}
	})()

	// 导航到目标页面
	if err := page.Context(ctx).Navigate(targetURL); err != nil {
		return "", nil, fmt.Errorf("导航失败: %w", err)
	}

	if err := page.Context(ctx).WaitLoad(); err != nil {
		return "", nil, fmt.Errorf("等待加载失败: %w", err)
	}

	page.Context(ctx).WaitIdle(5 * time.Second)

	html, err = page.HTML()
	if err != nil {
		return "", nil, fmt.Errorf("获取 HTML 失败: %w", err)
	}

	assetMu.Lock()
	assets = make([]string, len(assetURLs))
	copy(assets, assetURLs)
	assetMu.Unlock()

	return html, assets, nil
}

// Screenshot 对指定 URL 进行截图
func (c *Controller) Screenshot(ctx context.Context, targetURL string, fullPage bool) ([]byte, error) {
	page, err := c.acquirePage()
	if err != nil {
		return nil, err
	}
	defer c.releasePage(page)

	if err := page.Context(ctx).Navigate(targetURL); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	if err := page.Context(ctx).WaitLoad(); err != nil {
		return nil, fmt.Errorf("等待加载失败: %w", err)
	}

	var img []byte
	if fullPage {
		img, err = page.Context(ctx).Screenshot(true, &proto.PageCaptureScreenshot{})
	} else {
		img, err = page.Context(ctx).Screenshot(false, &proto.PageCaptureScreenshot{})
	}

	if err != nil {
		return nil, fmt.Errorf("截图失败: %w", err)
	}

	return img, nil
}

// EvaluateJS 在页面中执行 JavaScript
func (c *Controller) EvaluateJS(ctx context.Context, targetURL string, script string) (interface{}, error) {
	page, err := c.acquirePage()
	if err != nil {
		return nil, err
	}
	defer c.releasePage(page)

	if err := page.Context(ctx).Navigate(targetURL); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	if err := page.Context(ctx).WaitLoad(); err != nil {
		return nil, fmt.Errorf("等待加载失败: %w", err)
	}

	result, err := page.Context(ctx).Eval(script)
	if err != nil {
		return nil, fmt.Errorf("执行脚本失败: %w", err)
	}

	return result.Value, nil
}

// acquirePage 从池中获取页面
func (c *Controller) acquirePage() (*rod.Page, error) {
	select {
	case page := <-c.pagePool:
		return page, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("获取页面超时，页面池已满")
	}
}

// releasePage 将页面归还到池中
func (c *Controller) releasePage(page *rod.Page) {
	// 清理页面状态
	_ = page.Navigate("about:blank")

	select {
	case c.pagePool <- page:
	default:
		// 池已满，关闭页面
		page.Close()
	}
}

// loadCookies 从文件加载 Cookie
func (c *Controller) loadCookies(cookieFile string) error {
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var cookies []*proto.NetworkCookieParam

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 Netscape Cookie 格式
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}

		domain := parts[0]
		path := parts[2]
		secure := parts[3] == "TRUE"
		name := parts[5]
		value := parts[6]

		cookies = append(cookies, &proto.NetworkCookieParam{
			Domain: domain,
			Path:   path,
			Secure: secure,
			Name:   name,
			Value:  value,
		})
	}

	// 使用第一个页面设置 Cookie (通过 CDP 协议)
	select {
	case page := <-c.pagePool:
		for _, cookie := range cookies {
			// 使用 proto.NetworkSetCookie 设置单个 Cookie
			_, err := proto.NetworkSetCookie{
				Name:   cookie.Name,
				Value:  cookie.Value,
				Domain: cookie.Domain,
				Path:   cookie.Path,
				Secure: cookie.Secure,
			}.Call(page)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[警告] 设置 Cookie 失败 %s: %v\n", cookie.Name, err)
			}
		}
		c.pagePool <- page
	default:
		return fmt.Errorf("页面池为空，无法设置 Cookie")
	}

	return nil
}

// Close 关闭浏览器控制器
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// 关闭所有页面
	close(c.pagePool)
	for page := range c.pagePool {
		page.Close()
	}

	// 关闭浏览器
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
			return fmt.Errorf("关闭浏览器失败: %w", err)
		}
	}

	// 关闭启动器
	if c.launcher != nil {
		c.launcher.Cleanup()
	}

	return nil
}

// IsHealthy 检查浏览器是否健康
func (c *Controller) IsHealthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.browser == nil {
		return false
	}

	// 尝试获取浏览器版本作为健康检查
	_, err := c.browser.Version()
	return err == nil
}

// PageCount 返回当前可用页面数
func (c *Controller) PageCount() int {
	return len(c.pagePool)
}

// EnsureOutputDir 确保输出目录存在
func EnsureOutputDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", dir, err)
	}
	return nil
}

// SaveScreenshot 保存截图到文件
func SaveScreenshot(data []byte, dir string, filename string) error {
	if err := EnsureOutputDir(dir); err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	return os.WriteFile(path, data, 0644)
}
