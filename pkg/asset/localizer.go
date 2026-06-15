// Package asset 提供 CSS、图片、字体等资源的下载与本地化功能。
// 将外部资源引用转换为本地路径，确保离线可用。
package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Config 定义资源本地化配置
type Config struct {
	OutputDir      string   // 输出根目录
	AssetDir       string   // 资源存放子目录（如 "assets"）
	MaxFileSize    int64    // 最大文件大小（字节），0 表示不限制
	AllowedTypes   []string // 允许下载的 MIME 类型白名单
	SkipDataURI    bool     // 跳过 data URI
	RewriteURL     bool     // 重写 URL 为本地路径
	PreserveStructure bool  // 保持原始目录结构
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		OutputDir:      "./shadow-output",
		AssetDir:       "assets",
		MaxFileSize:    50 * 1024 * 1024, // 50MB
		AllowedTypes:   nil,              // nil 表示允许所有类型
		SkipDataURI:    true,
		RewriteURL:     true,
		PreserveStructure: false,
	}
}

// Localizer 是资源本地化器
type Localizer struct {
	config    Config
	client    *http.Client
	mu        sync.RWMutex
	urlMap    map[string]string // 原始 URL -> 本地路径
}

// NewLocalizer 创建新的资源本地化器
func NewLocalizer(cfg Config) *Localizer {
	return &Localizer{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		urlMap: make(map[string]string),
	}
}

// LocalizeDocument 本地化 HTML 文档中的所有资源引用
func (l *Localizer) LocalizeDocument(doc *html.Node, baseURL string) error {
	base, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("解析基础 URL 失败: %w", err)
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				traverse(c)
			}
			return
		}

		// 处理不同标签的资源属性
		switch n.Data {
		case "img":
			l.localizeAttr(n, "src", base)
			l.localizeAttr(n, "srcset", base) // 处理响应式图片
			l.localizeAttr(n, "data-src", base)
		case "script":
			l.localizeAttr(n, "src", base)
		case "link":
			// 只处理样式表和图标
			if l.getAttr(n, "rel") == "stylesheet" || l.getAttr(n, "rel") == "icon" || l.getAttr(n, "rel") == "shortcut icon" {
				l.localizeAttr(n, "href", base)
			}
		case "video", "audio", "source":
			l.localizeAttr(n, "src", base)
		case "iframe":
			l.localizeAttr(n, "src", base)
		case "object":
			l.localizeAttr(n, "data", base)
		case "embed":
			l.localizeAttr(n, "src", base)
		}

		// 处理内联样式中的背景图片
		if style := l.getAttr(n, "style"); style != "" {
			newStyle := l.localizeCSSURLs(style, base)
			l.setAttr(n, "style", newStyle)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return nil
}

// localizeAttr 本地化指定属性中的 URL
func (l *Localizer) localizeAttr(n *html.Node, attrName string, base *url.URL) {
	val := l.getAttr(n, attrName)
	if val == "" {
		return
	}

	// 处理 srcset（响应式图片）
	if attrName == "srcset" {
		newSrcset := l.localizeSrcset(val, base)
		l.setAttr(n, attrName, newSrcset)
		return
	}

	// 跳过 data URI
	if strings.HasPrefix(val, "data:") {
		if l.config.SkipDataURI {
			return
		}
		// TODO: 可选的 data URI 解码保存
		return
	}

	// 解析并下载资源
	resolved := resolveURL(base, val)
	if resolved == "" {
		return
	}

	localPath, err := l.downloadAsset(resolved)
	if err != nil {
		// 下载失败，保留原始 URL
		return
	}

	if l.config.RewriteURL {
		l.setAttr(n, attrName, localPath)
	}
}

// localizeSrcset 处理响应式图片的 srcset 属性
func (l *Localizer) localizeSrcset(srcset string, base *url.URL) string {
	parts := strings.Split(srcset, ",")
	var newParts []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}

		imgURL := fields[0]
		descriptor := ""
		if len(fields) > 1 {
			descriptor = fields[1]
		}

		if strings.HasPrefix(imgURL, "data:") {
			newParts = append(newParts, part)
			continue
		}

		resolved := resolveURL(base, imgURL)
		if resolved == "" {
			newParts = append(newParts, part)
			continue
		}

		localPath, err := l.downloadAsset(resolved)
		if err != nil {
			newParts = append(newParts, part)
			continue
		}

		if descriptor != "" {
			newParts = append(newParts, localPath+" "+descriptor)
		} else {
			newParts = append(newParts, localPath)
		}
	}

	return strings.Join(newParts, ", ")
}

// localizeCSSURLs 本地化 CSS 文本中的 url() 引用
func (l *Localizer) localizeCSSURLs(css string, base *url.URL) string {
	// 使用正则匹配 url(...)
	re := cssURLRegex
	return re.ReplaceAllStringFunc(css, func(match string) string {
		// 提取 URL
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		assetURL := strings.Trim(submatches[1], `"'`)
		if strings.HasPrefix(assetURL, "data:") {
			return match
		}

		resolved := resolveURL(base, assetURL)
		if resolved == "" {
			return match
		}

		localPath, err := l.downloadAsset(resolved)
		if err != nil {
			return match
		}

		// 保持原始引号风格
		if strings.Contains(match, `"`) {
			return `url("` + localPath + `")`
		} else if strings.Contains(match, `'`) {
			return `url('` + localPath + `')`
		}
		return `url(` + localPath + `)`
	})
}

// downloadAsset 下载资源并保存到本地
func (l *Localizer) downloadAsset(assetURL string) (string, error) {
	// 检查是否已下载
	l.mu.RLock()
	if localPath, ok := l.urlMap[assetURL]; ok {
		l.mu.RUnlock()
		return localPath, nil
	}
	l.mu.RUnlock()

	// 下载资源
	resp, err := l.client.Get(assetURL)
	if err != nil {
		return "", fmt.Errorf("下载资源失败 %s: %w", assetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, assetURL)
	}

	// 检查文件大小
	if l.config.MaxFileSize > 0 && resp.ContentLength > l.config.MaxFileSize {
		return "", fmt.Errorf("文件超过大小限制 %d > %d: %s", resp.ContentLength, l.config.MaxFileSize, assetURL)
	}

	// 检查 MIME 类型
	contentType := resp.Header.Get("Content-Type")
	if l.config.AllowedTypes != nil && len(l.config.AllowedTypes) > 0 {
		allowed := false
		for _, t := range l.config.AllowedTypes {
			if strings.Contains(contentType, t) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("不允许的 MIME 类型 %s: %s", contentType, assetURL)
		}
	}

	// 确定本地文件路径
	localPath := l.generateLocalPath(assetURL, contentType)
	fullPath := filepath.Join(l.config.OutputDir, l.config.AssetDir, localPath)

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 保存文件
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	// 记录映射关系
	relativePath := filepath.Join(l.config.AssetDir, localPath)
	// 统一使用正斜杠
	relativePath = strings.ReplaceAll(relativePath, "\\", "/")

	l.mu.Lock()
	l.urlMap[assetURL] = relativePath
	l.mu.Unlock()

	return relativePath, nil
}

// generateLocalPath 根据 URL 和 MIME 类型生成本地文件路径
func (l *Localizer) generateLocalPath(assetURL string, contentType string) string {
	u, err := url.Parse(assetURL)
	if err != nil {
		// 使用哈希作为文件名
		hash := sha256.Sum256([]byte(assetURL))
		return hex.EncodeToString(hash[:8]) + ".bin"
	}

	if l.config.PreserveStructure {
		// 保持原始目录结构
		path := u.Path
		if path == "" || path == "/" {
			path = "index"
		}
		if filepath.Ext(path) == "" {
			ext := extensionFromMIME(contentType)
			if ext != "" {
				path += ext
			}
		}
		return path
	}

	// 使用哈希命名，避免冲突
	hash := sha256.Sum256([]byte(assetURL))
	hashStr := hex.EncodeToString(hash[:8])

	// 尝试保留原始扩展名
	ext := filepath.Ext(u.Path)
	if ext == "" {
		ext = extensionFromMIME(contentType)
	}

	// 按扩展名分类存放
	if ext != "" {
		return path.Join(ext[1:], hashStr+ext)
	}

	return hashStr + ".bin"
}

// extensionFromMIME 根据 MIME 类型获取文件扩展名
func extensionFromMIME(contentType string) string {
	exts, err := mime.ExtensionsByType(contentType)
	if err == nil && len(exts) > 0 {
		return exts[0]
	}

	// 常见类型的回退映射
	mappings := map[string]string{
		"text/css":                  ".css",
		"text/javascript":           ".js",
		"application/javascript":    ".js",
		"image/jpeg":                ".jpg",
		"image/png":                 ".png",
		"image/gif":                 ".gif",
		"image/svg+xml":             ".svg",
		"image/webp":                ".webp",
		"image/x-icon":              ".ico",
		"font/woff2":                ".woff2",
		"font/woff":                 ".woff",
		"font/ttf":                  ".ttf",
		"font/otf":                  ".otf",
		"application/font-woff":     ".woff",
		"application/x-font-ttf":    ".ttf",
		"application/octet-stream":  ".bin",
	}

	// 去除 charset 等参数
	contentType = strings.Split(contentType, ";")[0]
	if ext, ok := mappings[contentType]; ok {
		return ext
	}

	return ""
}

// resolveURL 解析相对 URL
func resolveURL(base *url.URL, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "tel:") {
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

// getAttr 获取节点属性值
func (l *Localizer) getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// setAttr 设置节点属性值
func (l *Localizer) setAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// GetLocalPath 查询已下载资源的本地路径
func (l *Localizer) GetLocalPath(originalURL string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	path, ok := l.urlMap[originalURL]
	return path, ok
}

// GetURLMap 获取所有 URL 映射的副本
func (l *Localizer) GetURLMap() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copy := make(map[string]string, len(l.urlMap))
	for k, v := range l.urlMap {
		copy[k] = v
	}
	return copy
}

// cssURLRegex 用于匹配 CSS 中的 url()
var cssURLRegex = regexp.MustCompile(`url\(["']?([^"')]+)["']?\)`)
