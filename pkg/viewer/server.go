// Package viewer 提供本地 HTTP 服务器，用于浏览已克隆的网站内容或 ZIM 文件。
package viewer

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config 定义服务器配置
type Config struct {
	Addr        string        // 监听地址
	Port        int           // 监听端口
	RootDir     string        // 服务根目录
	ZIMFile     string        // ZIM 文件路径（与 RootDir 二选一）
	ReadTimeout time.Duration // 读取超时
	IndexFile   string        // 默认索引文件
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Addr:        "127.0.0.1",
		Port:        8080,
		ReadTimeout: 30 * time.Second,
		IndexFile:   "index.html",
	}
}

// Server 是本地 HTTP 服务器
type Server struct {
	config Config
	server *http.Server
	mu     sync.Mutex
	closed bool
}

// NewServer 创建新的服务器
func NewServer(cfg Config) (*Server, error) {
	if cfg.RootDir == "" && cfg.ZIMFile == "" {
		return nil, fmt.Errorf("必须指定 RootDir 或 ZIMFile")
	}

	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.IndexFile == "" {
		cfg.IndexFile = "index.html"
	}

	s := &Server{
		config: cfg,
	}

	mux := http.NewServeMux()

	if cfg.ZIMFile != "" {
		// ZIM 模式
		mux.HandleFunc("/", s.handleZIMRequest)
	} else {
		// 目录服务模式
		mux.HandleFunc("/", s.handleDirRequest)
	}

	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.ReadTimeout,
	}

	return s, nil
}

// Start 启动服务器（阻塞）
func (s *Server) Start() error {
	fmt.Fprintf(os.Stderr, "[ShadowWeb] 服务器启动: http://%s:%d\n", s.config.Addr, s.config.Port)

	if s.config.RootDir != "" {
		absDir, _ := filepath.Abs(s.config.RootDir)
		fmt.Fprintf(os.Stderr, "[ShadowWeb] 服务目录: %s\n", absDir)
	}
	if s.config.ZIMFile != "" {
		absFile, _ := filepath.Abs(s.config.ZIMFile)
		fmt.Fprintf(os.Stderr, "[ShadowWeb] ZIM 文件: %s\n", absFile)
	}

	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器启动失败: %w", err)
	}
	return nil
}

// StartAsync 异步启动服务器
func (s *Server) StartAsync() error {
	go func() {
		if err := s.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[错误] 服务器异常退出: %v\n", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// handleDirRequest 处理目录服务请求
func (s *Server) handleDirRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	// 清理请求路径，防止目录遍历
	cleanPath := path.Clean(r.URL.Path)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "非法路径", http.StatusForbidden)
		return
	}

	// 构建文件路径
	filePath := filepath.Join(s.config.RootDir, filepath.FromSlash(cleanPath))

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 尝试添加 .html 扩展名
			if !strings.HasSuffix(filePath, ".html") {
				filePath += ".html"
				info, err = os.Stat(filePath)
				if err != nil {
					http.NotFound(w, r)
					return
				}
			} else {
				http.NotFound(w, r)
				return
			}
		} else {
			http.Error(w, "访问文件失败", http.StatusInternalServerError)
			return
		}
	}

	// 如果是目录，尝试查找索引文件
	if info.IsDir() {
		indexPath := filepath.Join(filePath, s.config.IndexFile)
		if _, err := os.Stat(indexPath); err == nil {
			filePath = indexPath
		} else {
			// 返回目录列表（简化版）
			s.serveDirectoryList(w, r, filePath)
			return
		}
	}

	// 设置 MIME 类型
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	// 设置缓存头
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// 发送文件
	http.ServeFile(w, r, filePath)
}

// handleZIMRequest 处理 ZIM 文件请求（简化实现）
func (s *Server) handleZIMRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：目前将 ZIM 作为静态文件提供下载
	// 完整实现需要解析 ZIM 格式并提供内部文件访问
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(s.config.ZIMFile)))

	file, err := os.Open(s.config.ZIMFile)
	if err != nil {
		http.Error(w, "无法打开 ZIM 文件", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	io.Copy(w, file)
}

// serveDirectoryList 提供目录列表
func (s *Server) serveDirectoryList(w http.ResponseWriter, r *http.Request, dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "读取目录失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!DOCTYPE html>\n<html>\n<head>\n")
	fmt.Fprintf(w, "<meta charset=\"utf-8\">\n")
	fmt.Fprintf(w, "<title>目录列表 - ShadowWeb</title>\n")
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, "body{font-family:sans-serif;max-width:800px;margin:40px auto;padding:0 20px}\n")
	fmt.Fprintf(w, "h1{border-bottom:1px solid #ddd;padding-bottom:10px}\n")
	fmt.Fprintf(w, "ul{list-style:none;padding:0}\n")
	fmt.Fprintf(w, "li{margin:8px 0}\n")
	fmt.Fprintf(w, "a{text-decoration:none;color:#0066cc}\n")
	fmt.Fprintf(w, "a:hover{text-decoration:underline}\n")
	fmt.Fprintf(w, ".dir{font-weight:bold}\n")
	fmt.Fprintf(w, "</style>\n</head>\n<body>\n")

	// 计算相对路径显示
	relPath, _ := filepath.Rel(s.config.RootDir, dirPath)
	if relPath == "." {
		relPath = "/"
	}
	fmt.Fprintf(w, "<h1>目录: %s</h1>\n", relPath)
	fmt.Fprintf(w, "<ul>\n")

	// 父目录链接
	if relPath != "/" {
		fmt.Fprintf(w, "<li><a href=\"../\">../</a></li>\n")
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		class := ""
		if entry.IsDir() {
			class = " class=\"dir\""
		}
		fmt.Fprintf(w, "<li><a href=\"%s\"%s>%s</a></li>\n", name, class, name)
	}

	fmt.Fprintf(w, "</ul>\n")
	fmt.Fprintf(w, "<hr><p><small>由 ShadowWeb 提供服务</small></p>\n")
	fmt.Fprintf(w, "</body>\n</html>\n")
}

// handleHealth 健康检查端点
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`+"\n", time.Now().Format(time.RFC3339))
}

// URL 返回服务器访问地址
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s:%d", s.config.Addr, s.config.Port)
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}
