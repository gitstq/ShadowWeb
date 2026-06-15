// Package pack 提供打包引擎，将克隆的网站内容打包为 ZIM 或其他归档格式。
package pack

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitstq/ShadowWeb/pkg/zim"
)

// Config 定义打包配置
type Config struct {
	SourceDir   string    // 源目录
	OutputPath  string    // 输出文件路径
	Format      string    // 打包格式：zim, tar, zip
	MainPage    string    // 主页文件名
	Title       string    // 包标题
	Description string    // 包描述
	Language    string    // 内容语言
	Date        time.Time // 创建日期
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Format:   "zim",
		MainPage: "index.html",
		Language: "zh",
		Date:     time.Now(),
	}
}

// Packer 是打包引擎
type Packer struct {
	config Config
}

// NewPacker 创建新的打包器
func NewPacker(cfg Config) (*Packer, error) {
	if cfg.SourceDir == "" {
		return nil, fmt.Errorf("源目录不能为空")
	}
	if cfg.OutputPath == "" {
		return nil, fmt.Errorf("输出路径不能为空")
	}
	if cfg.Format == "" {
		cfg.Format = "zim"
	}
	if cfg.MainPage == "" {
		cfg.MainPage = "index.html"
	}

	// 验证源目录
	info, err := os.Stat(cfg.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("无法访问源目录: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("源路径不是目录: %s", cfg.SourceDir)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	return &Packer{config: cfg}, nil
}

// Pack 执行打包
func (p *Packer) Pack() error {
	switch strings.ToLower(p.config.Format) {
	case "zim":
		return p.packZIM()
	case "tar":
		return p.packTAR()
	case "zip":
		return p.packZIP()
	default:
		return fmt.Errorf("不支持的打包格式: %s", p.config.Format)
	}
}

// packZIM 打包为 ZIM 格式
func (p *Packer) packZIM() error {
	zimCfg := zim.Config{
		Path:     p.config.OutputPath,
		MainPage: p.config.MainPage,
	}

	writer, err := zim.NewWriter(zimCfg)
	if err != nil {
		return fmt.Errorf("创建 ZIM 写入器失败: %w", err)
	}
	defer writer.Close()

	// 遍历源目录
	fileCount := 0
	err = filepath.Walk(p.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 跳过状态文件
		if info.Name() == "state.json" {
			return nil
		}

		// 计算相对路径作为 URL
		relPath, err := filepath.Rel(p.config.SourceDir, path)
		if err != nil {
			return err
		}

		// 使用正斜杠
		url := strings.ReplaceAll(relPath, "\\", "/")
		title := info.Name()

		if err := writer.AddFile(path, url, title); err != nil {
			fmt.Fprintf(os.Stderr, "[警告] 添加文件失败 %s: %v\n", path, err)
			return nil // 继续处理其他文件
		}

		fileCount++
		if fileCount%100 == 0 {
			fmt.Fprintf(os.Stderr, "[打包] 已处理 %d 个文件...\n", fileCount)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("遍历目录失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[打包] 共打包 %d 个文件到 %s\n", fileCount, p.config.OutputPath)
	return nil
}

// packTAR 打包为 TAR 格式
func (p *Packer) packTAR() error {
	file, err := os.Create(p.config.OutputPath)
	if err != nil {
		return fmt.Errorf("创建 TAR 文件失败: %w", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	fileCount := 0
	err = filepath.Walk(p.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 跳过状态文件
		if info.Name() == "state.json" {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(p.config.SourceDir, path)
		if err != nil {
			return err
		}

		// 创建 TAR 头
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = strings.ReplaceAll(relPath, "\\", "/")

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// 写入文件内容
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if _, err := tw.Write(data); err != nil {
			return err
		}

		fileCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("打包 TAR 失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[打包] 共打包 %d 个文件到 %s\n", fileCount, p.config.OutputPath)
	return nil
}

// packZIP 打包为 ZIP 格式
func (p *Packer) packZIP() error {
	file, err := os.Create(p.config.OutputPath)
	if err != nil {
		return fmt.Errorf("创建 ZIP 文件失败: %w", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	fileCount := 0
	err = filepath.Walk(p.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 跳过状态文件
		if info.Name() == "state.json" {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(p.config.SourceDir, path)
		if err != nil {
			return err
		}

		// 创建 ZIP 头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = strings.ReplaceAll(relPath, "\\", "/")
		header.Method = zip.Deflate // 使用压缩

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		// 写入文件内容
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if _, err := io.Copy(writer, srcFile); err != nil {
			return err
		}

		fileCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("打包 ZIP 失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[打包] 共打包 %d 个文件到 %s\n", fileCount, p.config.OutputPath)
	return nil
}

// Stats 返回打包统计信息
func (p *Packer) Stats() (sourceSize int64, fileCount int, err error) {
	var totalSize int64
	var count int

	err = filepath.Walk(p.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
			count++
		}
		return nil
	})

	return totalSize, count, err
}
