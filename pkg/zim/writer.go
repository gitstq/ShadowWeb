// Package zim 提供 ZIM 格式写入器。
// ZIM 是开放标准的离线文档格式，被 Kiwix 等阅读器支持。
package zim

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZIM 文件格式常量
const (
	ZIMMagicNumber   uint32 = 0x44D495A5
	ZIMVersionMajor  uint16 = 5
	ZIMVersionMinor  uint16 = 1

	// 集群压缩类型
	ClusterDefault  uint8 = 0
	ClusterNoComp   uint8 = 1
	ClusterLZMA     uint8 = 4
	ClusterZSTD     uint8 = 5

	// MIME 类型列表中的特殊值
	MIMERedirect    = 0xffff
	MIMELinkTarget  = 0xfffe
	MIMEDeleted     = 0xfffd
)

// Header 表示 ZIM 文件头
type Header struct {
	MagicNumber      uint32
	VersionMajor     uint16
	VersionMinor     uint16
	UUID             [16]byte
	ArticleCount     uint32
	ClusterCount     uint32
	URLPtrPos        uint64
	TitlePtrPos      uint64
	ClusterPtrPos    uint64
	MimeListPos      uint64
	MainPage         uint32
	LayoutPage       uint32
	ChecksumPos      uint64
	GeoIndexPos      uint64 // 版本 5.1+
}

// Entry 表示 ZIM 文件中的一个条目
type Entry struct {
	MIMETYPE    uint16
	ParameterLen uint8
	Namespace   byte
	Revision    uint32
	URL         string
	Title       string
	Content     []byte
	RedirectURL string // 仅用于重定向条目
}

// Writer 是 ZIM 文件写入器
type Writer struct {
	file        *os.File
	path        string
	entries     []Entry
	mimeTypes   []string
	mimeMap     map[string]uint16
	clusterSize int
	compression uint8
}

// Config 定义 ZIM 写入器配置
type Config struct {
	Path        string // 输出文件路径
	ClusterSize int    // 每个集群的最大条目数
	Compression uint8  // 压缩类型
	MainPage    string // 主页 URL
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		ClusterSize: 100,
		Compression: ClusterZSTD,
		MainPage:    "index.html",
	}
}

// NewWriter 创建新的 ZIM 写入器
func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("ZIM 文件路径不能为空")
	}

	if cfg.ClusterSize <= 0 {
		cfg.ClusterSize = 100
	}

	// 确保输出目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	file, err := os.Create(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("创建 ZIM 文件失败: %w", err)
	}

	writer := &Writer{
		file:        file,
		path:        cfg.Path,
		entries:     make([]Entry, 0),
		mimeTypes:   make([]string, 0),
		mimeMap:     make(map[string]uint16),
		clusterSize: cfg.ClusterSize,
		compression: cfg.Compression,
	}

	// 注册默认 MIME 类型
	writer.registerMIMEType("text/html")
	writer.registerMIMEType("text/css")
	writer.registerMIMEType("text/javascript")
	writer.registerMIMEType("application/javascript")
	writer.registerMIMEType("image/jpeg")
	writer.registerMIMEType("image/png")
	writer.registerMIMEType("image/gif")
	writer.registerMIMEType("image/svg+xml")
	writer.registerMIMEType("image/webp")
	writer.registerMIMEType("image/x-icon")
	writer.registerMIMEType("font/woff2")
	writer.registerMIMEType("font/woff")
	writer.registerMIMEType("font/ttf")
	writer.registerMIMEType("application/json")
	writer.registerMIMEType("application/xml")
	writer.registerMIMEType("text/plain")

	return writer, nil
}

// registerMIMEType 注册 MIME 类型并返回其 ID
func (w *Writer) registerMIMEType(mime string) uint16 {
	if id, ok := w.mimeMap[mime]; ok {
		return id
	}

	id := uint16(len(w.mimeTypes))
	w.mimeTypes = append(w.mimeTypes, mime)
	w.mimeMap[mime] = id
	return id
}

// AddEntry 添加一个条目到 ZIM 文件
func (w *Writer) AddEntry(url, title, mimeType string, content []byte) error {
	if url == "" {
		return fmt.Errorf("条目 URL 不能为空")
	}

	// 规范化 URL
	url = strings.TrimPrefix(url, "/")
	if url == "" {
		url = "index.html"
	}

	mimeID := w.registerMIMEType(mimeType)

	entry := Entry{
		MIMETYPE: mimeID,
		Namespace: 'A', // A = 文章内容
		Revision:  uint32(time.Now().Unix()),
		URL:       url,
		Title:     title,
		Content:   content,
	}

	w.entries = append(w.entries, entry)
	return nil
}

// AddRedirect 添加重定向条目
func (w *Writer) AddRedirect(url, title, targetURL string) error {
	if url == "" {
		return fmt.Errorf("重定向 URL 不能为空")
	}

	url = strings.TrimPrefix(url, "/")
	targetURL = strings.TrimPrefix(targetURL, "/")

	entry := Entry{
		MIMETYPE:    MIMERedirect,
		Namespace:   'A',
		Revision:    uint32(time.Now().Unix()),
		URL:         url,
		Title:       title,
		RedirectURL: targetURL,
	}

	w.entries = append(w.entries, entry)
	return nil
}

// AddFile 从文件系统添加文件
func (w *Writer) AddFile(filePath, url, title string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败 %s: %w", filePath, err)
	}

	// 根据扩展名推断 MIME 类型
	mimeType := inferMIMEType(filePath)

	return w.AddEntry(url, title, mimeType, data)
}

// AddDirectory 递归添加目录中的所有文件
func (w *Writer) AddDirectory(dirPath, baseURL string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 计算相对路径作为 URL
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// 使用正斜杠
		url := filepath.Join(baseURL, relPath)
		url = strings.ReplaceAll(url, "\\", "/")

		title := info.Name()
		return w.AddFile(path, url, title)
	})
}

// Close 完成 ZIM 文件写入并关闭
func (w *Writer) Close() error {
	if w.file == nil {
		return nil
	}

	defer func() {
		w.file.Close()
		w.file = nil
	}()

	// 写入文件内容
	if err := w.writeFile(); err != nil {
		return fmt.Errorf("写入 ZIM 文件失败: %w", err)
	}

	return nil
}

// writeFile 写入 ZIM 文件格式
func (w *Writer) writeFile() error {
	// 预留文件头空间
	headerSize := int64(80) // 基础头大小
	if _, err := w.file.Seek(headerSize, io.SeekStart); err != nil {
		return err
	}

	// 写入 MIME 类型列表
	mimeListPos, err := w.writeMIMEList()
	if err != nil {
		return fmt.Errorf("写入 MIME 列表失败: %w", err)
	}

	// 按 URL 排序条目（ZIM 要求）
	w.sortEntries()

	// 写入条目内容到集群
	clusterPtrPos, clusterOffsets, err := w.writeClusters()
	if err != nil {
		return fmt.Errorf("写入集群失败: %w", err)
	}

	// 写入 URL 指针列表
	urlPtrPos, _, err := w.writeURLPointers(clusterOffsets)
	if err != nil {
		return fmt.Errorf("写入 URL 指针失败: %w", err)
	}

	// 写入标题指针列表
	titlePtrPos, err := w.writeTitlePointers()
	if err != nil {
		return fmt.Errorf("写入标题指针失败: %w", err)
	}

	// 计算主页索引
	mainPage := uint32(0xffffffff)
	for i, entry := range w.entries {
		if entry.URL == "index.html" || entry.URL == "A/index.html" {
			mainPage = uint32(i)
			break
		}
	}

	// 计算校验和位置
	checksumPos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// 计算并写入校验和（简化：使用文件大小作为占位）
	if err := w.writeChecksum(); err != nil {
		return fmt.Errorf("写入校验和失败: %w", err)
	}

	// 回到文件开头写入头信息
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	header := Header{
		MagicNumber:   ZIMMagicNumber,
		VersionMajor:  ZIMVersionMajor,
		VersionMinor:  ZIMVersionMinor,
		ArticleCount:  uint32(len(w.entries)),
		ClusterCount:  uint32(len(clusterOffsets)),
		URLPtrPos:     uint64(urlPtrPos),
		TitlePtrPos:   uint64(titlePtrPos),
		ClusterPtrPos: uint64(clusterPtrPos),
		MimeListPos:   uint64(mimeListPos),
		MainPage:      mainPage,
		LayoutPage:    0xffffffff,
		ChecksumPos:   uint64(checksumPos),
		GeoIndexPos:   0,
	}

	if err := binary.Write(w.file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("写入文件头失败: %w", err)
	}

	return nil
}

// writeMIMEList 写入 MIME 类型列表
func (w *Writer) writeMIMEList() (int64, error) {
	pos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	for _, mime := range w.mimeTypes {
		if _, err := w.file.WriteString(mime + "\x00"); err != nil {
			return 0, err
		}
	}

	return pos, nil
}

// writeClusters 将条目内容写入集群
func (w *Writer) writeClusters() (int64, []int64, error) {
	pos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, nil, err
	}

	var clusterOffsets []int64
	var currentCluster []Entry

	for i, entry := range w.entries {
		currentCluster = append(currentCluster, entry)

		// 达到集群大小限制或最后一个条目
		if len(currentCluster) >= w.clusterSize || i == len(w.entries)-1 {
			offset, err := w.writeCluster(currentCluster)
			if err != nil {
				return 0, nil, err
			}
			clusterOffsets = append(clusterOffsets, offset)
			currentCluster = currentCluster[:0]
		}
	}

	return pos, clusterOffsets, nil
}

// writeCluster 写入单个集群
func (w *Writer) writeCluster(entries []Entry) (int64, error) {
	pos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// 集群头：压缩类型
	if _, err := w.file.Write([]byte{w.compression}); err != nil {
		return 0, err
	}

	// 对于无压缩，直接写入内容
	if w.compression == ClusterNoComp {
		// 写入偏移量数量
		count := uint32(len(entries))
		if err := binary.Write(w.file, binary.LittleEndian, count); err != nil {
			return 0, err
		}

		// 计算并写入偏移量
		var offsets []uint64
		var contentBuf bytes.Buffer
		currentOffset := uint64(4 + 8*len(entries)) // 跳过 count 和 offsets

		for _, entry := range entries {
			offsets = append(offsets, currentOffset)
			contentBuf.Write(entry.Content)
			currentOffset += uint64(len(entry.Content))
		}

		for _, offset := range offsets {
			if err := binary.Write(w.file, binary.LittleEndian, offset); err != nil {
				return 0, err
			}
		}

		if _, err := w.file.Write(contentBuf.Bytes()); err != nil {
			return 0, err
		}
	} else {
		// 其他压缩类型：先收集内容，再压缩写入
		// 简化实现：目前按无压缩处理
		count := uint32(len(entries))
		if err := binary.Write(w.file, binary.LittleEndian, count); err != nil {
			return 0, err
		}

		var offsets []uint64
		var contentBuf bytes.Buffer
		currentOffset := uint64(4 + 8*len(entries))

		for _, entry := range entries {
			offsets = append(offsets, currentOffset)
			contentBuf.Write(entry.Content)
			currentOffset += uint64(len(entry.Content))
		}

		for _, offset := range offsets {
			if err := binary.Write(w.file, binary.LittleEndian, offset); err != nil {
				return 0, err
			}
		}

		if _, err := w.file.Write(contentBuf.Bytes()); err != nil {
			return 0, err
		}
	}

	return pos, nil
}

// writeURLPointers 写入 URL 指针列表
func (w *Writer) writeURLPointers(clusterOffsets []int64) (int64, []int64, error) {
	pos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, nil, err
	}

	var entryOffsets []int64
	clusterIdx := 0
	entryInCluster := 0

	for range w.entries {
		entryOffsets = append(entryOffsets, clusterOffsets[clusterIdx])
		entryInCluster++
		if entryInCluster >= w.clusterSize {
			clusterIdx++
			entryInCluster = 0
		}
	}

	for _, offset := range entryOffsets {
		if err := binary.Write(w.file, binary.LittleEndian, uint64(offset)); err != nil {
			return 0, nil, err
		}
	}

	return pos, entryOffsets, nil
}

// writeTitlePointers 写入标题指针列表
func (w *Writer) writeTitlePointers() (int64, error) {
	pos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// 按标题排序的索引
	titleOrder := make([]int, len(w.entries))
	for i := range titleOrder {
		titleOrder[i] = i
	}

	// 简单的冒泡排序（条目数通常不大）
	for i := 0; i < len(titleOrder); i++ {
		for j := i + 1; j < len(titleOrder); j++ {
			if w.entries[titleOrder[i]].Title > w.entries[titleOrder[j]].Title {
				titleOrder[i], titleOrder[j] = titleOrder[j], titleOrder[i]
			}
		}
	}

	for _, idx := range titleOrder {
		if err := binary.Write(w.file, binary.LittleEndian, uint32(idx)); err != nil {
			return 0, err
		}
	}

	return pos, nil
}

// writeChecksum 写入校验和
func (w *Writer) writeChecksum() error {
	// 简化实现：计算文件内容的 FNV-1a 哈希
	h := fnv.New128a()

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if _, err := io.Copy(h, w.file); err != nil {
		return err
	}

	checksum := h.Sum(nil)
	if _, err := w.file.Write(checksum); err != nil {
		return err
	}

	return nil
}

// sortEntries 按 URL 排序条目
func (w *Writer) sortEntries() {
	// 使用简单的选择排序
	for i := 0; i < len(w.entries); i++ {
		minIdx := i
		for j := i + 1; j < len(w.entries); j++ {
			if w.entries[j].URL < w.entries[minIdx].URL {
				minIdx = j
			}
		}
		w.entries[i], w.entries[minIdx] = w.entries[minIdx], w.entries[i]
	}
}

// inferMIMEType 根据文件扩展名推断 MIME 类型
func inferMIMEType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	mappings := map[string]string{
		".html":  "text/html",
		".htm":   "text/html",
		".css":   "text/css",
		".js":    "application/javascript",
		".json":  "application/json",
		".xml":   "application/xml",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".png":   "image/png",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".webp":  "image/webp",
		".ico":   "image/x-icon",
		".woff2": "font/woff2",
		".woff":  "font/woff",
		".ttf":   "font/ttf",
		".otf":   "font/otf",
		".eot":   "application/vnd.ms-fontobject",
		".pdf":   "application/pdf",
		".txt":   "text/plain",
		".md":    "text/markdown",
	}

	if mime, ok := mappings[ext]; ok {
		return mime
	}

	return "application/octet-stream"
}

// EntryCount 返回已添加的条目数
func (w *Writer) EntryCount() int {
	return len(w.entries)
}

// Size 返回当前文件大小
func (w *Writer) Size() (int64, error) {
	if w.file == nil {
		return 0, fmt.Errorf("文件未打开")
	}

	info, err := w.file.Stat()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}
