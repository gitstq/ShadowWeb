// Package index 提供基于 Bleve 的全文搜索索引构建与查询功能。
// 支持中文内容索引和多种查询方式。
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
)

// Document 表示一个可索引的文档
type Document struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	URL         string    `json:"url"`
	LocalPath   string    `json:"local_path"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// SearchResult 表示搜索结果
type SearchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	URL      string  `json:"url"`
	LocalPath string `json:"local_path"`
	Score    float64 `json:"score"`
	Excerpt  string  `json:"excerpt"`
}

// SearchResults 表示搜索结果集
type SearchResults struct {
	Total    int            `json:"total"`
	Hits     []SearchResult `json:"hits"`
	Duration time.Duration  `json:"duration"`
}

// Indexer 是全文索引器
type Indexer struct {
	index   bleve.Index
	path    string
	mu      sync.RWMutex
	closed  bool
}

// Config 定义索引器配置
type Config struct {
	IndexPath string // 索引存储路径
}

// NewIndexer 创建新的索引器
func NewIndexer(cfg Config) (*Indexer, error) {
	if cfg.IndexPath == "" {
		return nil, fmt.Errorf("索引路径不能为空")
	}

	// 确保索引目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.IndexPath), 0755); err != nil {
		return nil, fmt.Errorf("创建索引目录失败: %w", err)
	}

	var index bleve.Index
	var err error

	// 检查索引是否已存在
	if _, statErr := os.Stat(cfg.IndexPath); os.IsNotExist(statErr) {
		// 创建新索引
		mapping := buildIndexMapping()
		index, err = bleve.New(cfg.IndexPath, mapping)
		if err != nil {
			return nil, fmt.Errorf("创建索引失败: %w", err)
		}
	} else {
		// 打开已有索引
		index, err = bleve.Open(cfg.IndexPath)
		if err != nil {
			return nil, fmt.Errorf("打开索引失败: %w", err)
		}
	}

	return &Indexer{
		index: index,
		path:  cfg.IndexPath,
	}, nil
}

// buildIndexMapping 构建索引映射
func buildIndexMapping() mapping.IndexMapping {
	// 创建文档映射
	docMapping := bleve.NewDocumentMapping()

	// 标题字段 - 高权重
	titleMapping := bleve.NewTextFieldMapping()
	titleMapping.Analyzer = standard.Name
	titleMapping.Store = true
	titleMapping.Index = true
	docMapping.AddFieldMappingsAt("title", titleMapping)

	// 内容字段
	contentMapping := bleve.NewTextFieldMapping()
	contentMapping.Analyzer = standard.Name
	contentMapping.Store = true
	contentMapping.Index = true
	docMapping.AddFieldMappingsAt("content", contentMapping)

	// URL 字段 - 仅存储
	urlMapping := bleve.NewTextFieldMapping()
	urlMapping.Store = true
	urlMapping.Index = false
	docMapping.AddFieldMappingsAt("url", urlMapping)

	// 本地路径字段 - 仅存储
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Store = true
	pathMapping.Index = false
	docMapping.AddFieldMappingsAt("local_path", pathMapping)

	// 描述字段
	descMapping := bleve.NewTextFieldMapping()
	descMapping.Analyzer = standard.Name
	descMapping.Store = true
	descMapping.Index = true
	docMapping.AddFieldMappingsAt("description", descMapping)

	// 关键词字段
	keywordMapping := bleve.NewTextFieldMapping()
	keywordMapping.Analyzer = standard.Name
	keywordMapping.Store = true
	keywordMapping.Index = true
	docMapping.AddFieldMappingsAt("keywords", keywordMapping)

	// 创建索引映射
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("document", docMapping)
	indexMapping.TypeField = "type"
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// IndexDocument 索引单个文档
func (i *Indexer) IndexDocument(doc Document) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.closed {
		return fmt.Errorf("索引器已关闭")
	}

	if doc.ID == "" {
		return fmt.Errorf("文档 ID 不能为空")
	}

	doc.IndexedAt = time.Now()

	if err := i.index.Index(doc.ID, doc); err != nil {
		return fmt.Errorf("索引文档失败: %w", err)
	}

	return nil
}

// IndexDocuments 批量索引文档
func (i *Indexer) IndexDocuments(docs []Document) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.closed {
		return fmt.Errorf("索引器已关闭")
	}

	batch := i.index.NewBatch()
	for _, doc := range docs {
		if doc.ID == "" {
			continue
		}
		doc.IndexedAt = time.Now()
		if err := batch.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("添加文档到批次失败: %w", err)
		}
	}

	if err := i.index.Batch(batch); err != nil {
		return fmt.Errorf("批量索引失败: %w", err)
	}

	return nil
}

// Search 执行搜索查询
func (i *Indexer) Search(query string, limit int, offset int) (*SearchResults, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.closed {
		return nil, fmt.Errorf("索引器已关闭")
	}

	if query == "" {
		return &SearchResults{Total: 0, Hits: []SearchResult{}}, nil
	}

	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	start := time.Now()

	// 构建查询
	// 在标题、内容和描述中搜索
	q := bleve.NewQueryStringQuery(query)

	// 创建搜索请求
	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = limit
	searchRequest.From = offset
	searchRequest.Fields = []string{"title", "url", "local_path", "content", "description"}
	searchRequest.Highlight = bleve.NewHighlight()

	// 执行搜索
	searchResult, err := i.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	results := &SearchResults{
		Total:    int(searchResult.Total),
		Duration: time.Since(start),
		Hits:     make([]SearchResult, 0, len(searchResult.Hits)),
	}

	for _, hit := range searchResult.Hits {
		result := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}

		if title, ok := hit.Fields["title"].(string); ok {
			result.Title = title
		}
		if url, ok := hit.Fields["url"].(string); ok {
			result.URL = url
		}
		if localPath, ok := hit.Fields["local_path"].(string); ok {
			result.LocalPath = localPath
		}

		// 生成摘要
		if content, ok := hit.Fields["content"].(string); ok {
			result.Excerpt = generateExcerpt(content, query, 200)
		} else if desc, ok := hit.Fields["description"].(string); ok {
			result.Excerpt = desc
		}

		results.Hits = append(results.Hits, result)
	}

	return results, nil
}

// SearchByTitle 仅在标题中搜索
func (i *Indexer) SearchByTitle(title string, limit int) (*SearchResults, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.closed {
		return nil, fmt.Errorf("索引器已关闭")
	}

	start := time.Now()

	q := bleve.NewMatchQuery(title)
	q.SetField("title")

	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"title", "url", "local_path", "description"}

	searchResult, err := i.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	results := &SearchResults{
		Total:    int(searchResult.Total),
		Duration: time.Since(start),
		Hits:     make([]SearchResult, 0, len(searchResult.Hits)),
	}

	for _, hit := range searchResult.Hits {
		result := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}

		if title, ok := hit.Fields["title"].(string); ok {
			result.Title = title
		}
		if url, ok := hit.Fields["url"].(string); ok {
			result.URL = url
		}
		if localPath, ok := hit.Fields["local_path"].(string); ok {
			result.LocalPath = localPath
		}
		if desc, ok := hit.Fields["description"].(string); ok {
			result.Excerpt = desc
		}

		results.Hits = append(results.Hits, result)
	}

	return results, nil
}

// DeleteDocument 从索引中删除文档
func (i *Indexer) DeleteDocument(id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.closed {
		return fmt.Errorf("索引器已关闭")
	}

	if err := i.index.Delete(id); err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}

	return nil
}

// Count 返回索引中的文档总数
func (i *Indexer) Count() (uint64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.closed {
		return 0, fmt.Errorf("索引器已关闭")
	}

	return i.index.DocCount()
}

// Close 关闭索引器
func (i *Indexer) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.closed {
		return nil
	}

	i.closed = true
	return i.index.Close()
}

// generateExcerpt 生成包含搜索关键词的摘要
func generateExcerpt(content, query string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	// 将查询拆分为关键词
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return content[:maxLen] + "..."
	}

	// 查找第一个匹配关键词的位置
	lowerContent := strings.ToLower(content)
	bestPos := -1

	for _, kw := range keywords {
		if pos := strings.Index(lowerContent, kw); pos != -1 {
			if bestPos == -1 || pos < bestPos {
				bestPos = pos
			}
		}
	}

	if bestPos == -1 {
		// 没有找到关键词，返回开头
		return content[:maxLen] + "..."
	}

	// 计算摘要范围
	start := bestPos - maxLen/4
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	excerpt := content[start:end]
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(content) {
		excerpt = excerpt + "..."
	}

	return excerpt
}

// ExtractTextFromHTML 从 HTML 中提取纯文本（简化版）
func ExtractTextFromHTML(htmlStr string) string {
	// 移除 script 和 style 标签及其内容
	text := htmlStr

	// 简单替换常见标签
	replacements := map[string]string{
		"<br>":   "\n",
		"<br/>":  "\n",
		"<br />": "\n",
		"<p>":    "\n",
		"</p>":   "\n",
		"<div>":  "\n",
		"</div>": "\n",
		"<li>":   "\n- ",
		"</li>":  "\n",
		"<h1>":   "\n",
		"</h1>":  "\n",
		"<h2>":   "\n",
		"</h2>":  "\n",
		"<h3>":   "\n",
		"</h3>":  "\n",
		"<h4>":   "\n",
		"</h4>":  "\n",
		"<h5>":   "\n",
		"</h5>":  "\n",
		"<h6>":   "\n",
		"</h6>":  "\n",
	}

	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	// 移除所有 HTML 标签
	inTag := false
	var result strings.Builder
	for _, r := range text {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// 规范化空白
	text = result.String()
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}
