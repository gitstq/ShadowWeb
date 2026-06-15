// Package sanitize 提供 JavaScript 剥离与内容清洗功能。
// 移除不需要的脚本、追踪器、广告和动态内容，生成干净的静态 HTML。
package sanitize

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Config 定义清洗配置
type Config struct {
	RemoveJS        bool     // 移除所有 JavaScript
	RemoveAds       bool     // 移除广告相关元素
	RemoveTracking  bool     // 移除追踪器
	RemoveComments  bool     // 移除 HTML 注释
	RemoveIframes   bool     // 移除 iframe
	RemoveObjects   bool     // 移除 object/embed
	MinifyCSS       bool     // 压缩 CSS
	PreserveForms   bool     // 保留表单元素
	AllowedDomains  []string // 允许保留的外部域名白名单
}

// DefaultConfig 返回默认清洗配置
func DefaultConfig() Config {
	return Config{
		RemoveJS:       true,
		RemoveAds:      true,
		RemoveTracking: true,
		RemoveComments: true,
		RemoveIframes:  true,
		RemoveObjects:  true,
		MinifyCSS:      true,
		PreserveForms:  false,
	}
}

// Cleaner 是内容清洗器
type Cleaner struct {
	config Config
}

// NewCleaner 创建新的清洗器
func NewCleaner(cfg Config) *Cleaner {
	return &Cleaner{config: cfg}
}

// CleanReader 从 Reader 读取并清洗 HTML
func (c *Cleaner) CleanReader(r io.Reader) (*html.Node, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	return c.CleanNode(doc), nil
}

// CleanString 清洗 HTML 字符串
func (c *Cleaner) CleanString(htmlStr string) (*html.Node, error) {
	return c.CleanReader(strings.NewReader(htmlStr))
}

// CleanNode 清洗 HTML 节点树
func (c *Cleaner) CleanNode(doc *html.Node) *html.Node {
	if doc == nil {
		return nil
	}

	// 第一步：移除不需要的标签
	c.removeUnwantedTags(doc)

	// 第二步：移除不需要的属性
	c.cleanAttributes(doc)

	// 第三步：移除空节点
	c.removeEmptyNodes(doc)

	// 第四步：移除 HTML 注释
	if c.config.RemoveComments {
		c.removeComments(doc)
	}

	return doc
}

// removeUnwantedTags 递归移除不需要的标签
func (c *Cleaner) removeUnwantedTags(n *html.Node) {
	if n.Type != html.ElementNode {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			c.removeUnwantedTags(child)
			child = next
		}
		return
	}

	shouldRemove := false

	switch n.Data {
	case "script":
		// 如果配置保留表单相关脚本，检查是否是表单验证脚本
		if c.config.RemoveJS {
			shouldRemove = !c.isFormScript(n)
		}
	case "noscript":
		if c.config.RemoveJS {
			// 将 noscript 内容提升到父级，然后移除 noscript 标签
			c.unwrapNode(n)
			return
		}
	case "style":
		if c.config.MinifyCSS {
			c.minifyCSS(n)
		}
	case "iframe":
		if c.config.RemoveIframes {
			shouldRemove = true
		}
	case "object", "embed", "applet":
		if c.config.RemoveObjects {
			shouldRemove = true
		}
	case "link":
		// 移除预加载、DNS 预解析等性能优化标签
		if c.config.RemoveJS {
			for _, attr := range n.Attr {
				if attr.Key == "rel" && (attr.Val == "preload" || attr.Val == "prefetch" || attr.Val == "dns-prefetch" || attr.Val == "preconnect") {
					shouldRemove = true
					break
				}
			}
		}
	}

	// 广告和追踪器检测
	if !shouldRemove && (c.config.RemoveAds || c.config.RemoveTracking) {
		shouldRemove = c.isAdOrTrackingElement(n)
	}

	if shouldRemove {
		c.removeNode(n)
		return
	}

	// 递归处理子节点
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		c.removeUnwantedTags(child)
		child = next
	}
}

// cleanAttributes 清理元素属性
func (c *Cleaner) cleanAttributes(n *html.Node) {
	if n.Type != html.ElementNode {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			c.cleanAttributes(child)
		}
		return
	}

	// 需要移除的属性列表
	attrsToRemove := []string{}

	for _, attr := range n.Attr {
		shouldRemove := false

		switch attr.Key {
		case "onclick", "ondblclick", "onmousedown", "onmouseup", "onmouseover",
			"onmousemove", "onmouseout", "onkeydown", "onkeypress", "onkeyup",
			"onfocus", "onblur", "onchange", "onsubmit", "onreset", "onselect",
			"onload", "onunload", "onerror", "onresize", "onscroll":
			if c.config.RemoveJS {
				shouldRemove = true
			}
		case "data-tracking", "data-analytics", "data-gtm", "data-ga":
			if c.config.RemoveTracking {
				shouldRemove = true
			}
		case "ping":
			// 移除 hyperlink auditing
			shouldRemove = true
		}

		// 检测事件监听器属性（以 on 开头）
		if strings.HasPrefix(attr.Key, "on") && c.config.RemoveJS {
			shouldRemove = true
		}

		if shouldRemove {
			attrsToRemove = append(attrsToRemove, attr.Key)
		}
	}

	// 移除标记的属性
	for _, key := range attrsToRemove {
		removeAttribute(n, key)
	}

	// 递归处理子节点
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.cleanAttributes(child)
	}
}

// removeEmptyNodes 移除空节点（没有内容和子节点的元素）
func (c *Cleaner) removeEmptyNodes(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}

	// 递归处理子节点（从后往前，便于删除）
	for child := n.LastChild; child != nil; {
		prev := child.PrevSibling
		c.removeEmptyNodes(child)
		child = prev
	}

	// 检查是否是空节点（保留某些标签）
	if n.FirstChild == nil && isRemovableEmptyTag(n.Data) {
		c.removeNode(n)
	}
}

// removeComments 移除 HTML 注释节点
func (c *Cleaner) removeComments(n *html.Node) {
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.CommentNode {
			c.removeNode(child)
		} else {
			c.removeComments(child)
		}
		child = next
	}
}

// isFormScript 检查脚本是否与表单相关
func (c *Cleaner) isFormScript(n *html.Node) bool {
	if !c.config.PreserveForms {
		return false
	}

	// 检查 script 的 id 或 class 是否包含 form 相关关键词
	for _, attr := range n.Attr {
		if attr.Key == "id" || attr.Key == "class" {
			lower := strings.ToLower(attr.Val)
			if strings.Contains(lower, "form") || strings.Contains(lower, "validate") {
				return true
			}
		}
	}

	// 检查内容是否包含表单验证相关代码
	if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
		content := strings.ToLower(n.FirstChild.Data)
		if strings.Contains(content, "validation") || strings.Contains(content, "validate") {
			return true
		}
	}

	return false
}

// isAdOrTrackingElement 检测广告或追踪元素
func (c *Cleaner) isAdOrTrackingElement(n *html.Node) bool {
	// 检查标签名
	adTags := map[string]bool{
		"ins": true, // Google AdSense
	}
	if adTags[n.Data] {
		return true
	}

	// 检查 id 和 class
	for _, attr := range n.Attr {
		if attr.Key != "id" && attr.Key != "class" {
			continue
		}

		val := strings.ToLower(attr.Val)

		// 广告关键词
		adKeywords := []string{
			"ad-", "ads-", "advertisement", "banner", "sponsored",
			"googlead", "adsense", "doubleclick", "outbrain", "taboola",
		}
		for _, kw := range adKeywords {
			if strings.Contains(val, kw) && c.config.RemoveAds {
				return true
			}
		}

		// 追踪关键词
		trackingKeywords := []string{
			"tracking", "analytics", "gtm-", "ga-", "metrics",
			"pixel", "beacon", "stat", "monitor",
		}
		for _, kw := range trackingKeywords {
			if strings.Contains(val, kw) && c.config.RemoveTracking {
				return true
			}
		}
	}

	// 检查外部资源域名
	if n.Data == "script" || n.Data == "img" || n.Data == "iframe" {
		for _, attr := range n.Attr {
			if attr.Key == "src" {
				if c.isBlockedDomain(attr.Val) {
					return true
				}
			}
		}
	}

	return false
}

// isBlockedDomain 检查域名是否在黑名单中
func (c *Cleaner) isBlockedDomain(src string) bool {
	blockedDomains := []string{
		"google-analytics.com",
		"googletagmanager.com",
		"doubleclick.net",
		"facebook.com/tr",
		"googleadservices.com",
		"googleads.g.doubleclick.net",
		"amazon-adsystem.com",
		"adsystem.amazon.com",
		"outbrain.com",
		"taboola.com",
		"scorecardresearch.com",
		"quantserve.com",
		"googlesyndication.com",
	}

	srcLower := strings.ToLower(src)
	for _, domain := range blockedDomains {
		if strings.Contains(srcLower, domain) {
			return true
		}
	}

	return false
}

// minifyCSS 压缩内联 CSS
func (c *Cleaner) minifyCSS(n *html.Node) {
	if n.FirstChild == nil || n.FirstChild.Type != html.TextNode {
		return
	}

	css := n.FirstChild.Data
	// 移除注释
	css = removeCSSComments(css)
	// 移除多余空白
	css = regexp.MustCompile(`\s+`).ReplaceAllString(css, " ")
	// 移除选择器后的多余空格
	css = regexp.MustCompile(`\s*([{}:;,])\s*`).ReplaceAllString(css, "$1")
	// 移除末尾分号
	css = regexp.MustCompile(`;\s*}`).ReplaceAllString(css, "}")

	n.FirstChild.Data = strings.TrimSpace(css)
}

// removeCSSComments 移除 CSS 注释
func removeCSSComments(css string) string {
	re := regexp.MustCompile(`/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`)
	return re.ReplaceAllString(css, "")
}

// removeNode 从 DOM 树中移除节点
func (c *Cleaner) removeNode(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

// unwrapNode 展开节点（保留子节点，移除当前节点）
func (c *Cleaner) unwrapNode(n *html.Node) {
	parent := n.Parent
	if parent == nil {
		return
	}

	// 将所有子节点移到父节点中当前节点之前
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		parent.InsertBefore(child, n)
		child = next
	}

	parent.RemoveChild(n)
}

// removeAttribute 从节点移除指定属性
func removeAttribute(n *html.Node, key string) {
	for i, attr := range n.Attr {
		if attr.Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}

// isRemovableEmptyTag 检查标签是否可以在为空时移除
func isRemovableEmptyTag(tag string) bool {
	// 这些标签即使为空也有意义，不应移除
	preserveTags := map[string]bool{
		"img": true, "br": true, "hr": true, "input": true,
		"meta": true, "link": true, "source": true, "area": true,
		"base": true, "col": true, "embed": true, "param": true,
		"track": true, "wbr": true,
	}
	return !preserveTags[tag]
}

// CleanHTMLString 是便捷函数，直接清洗 HTML 字符串并返回字符串
func CleanHTMLString(htmlStr string, cfg Config) (string, error) {
	cleaner := NewCleaner(cfg)
	doc, err := cleaner.CleanString(htmlStr)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("渲染 HTML 失败: %w", err)
	}

	return buf.String(), nil
}

// StripScriptsOnly 仅剥离 script 标签，保留其他内容
func StripScriptsOnly(htmlStr string) (string, error) {
	cfg := Config{RemoveJS: true, RemoveAds: false, RemoveTracking: false}
	return CleanHTMLString(htmlStr, cfg)
}
