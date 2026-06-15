// Package cli 提供 ShadowWeb 的命令行接口，基于 Cobra 框架构建。
// 包含 clone、serve、pack、open、index 五个子命令。
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// 版本信息变量，由 Makefile 在构建时注入
var (
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "unknown"
)

// 全局标志变量
var (
	// 通用标志
	outputDir   string // 输出目录
	verbose     bool   // 详细输出
	concurrency int    // 并发数

	// 克隆相关标志
	scopePrefix  string   // 作用域前缀过滤
	excludeList  []string // 排除模式列表
	rateLimit    float64  // 限速（请求/秒）
	cookieFile   string   // Cookie 文件路径
	noJS         bool     // 是否剥离 JavaScript
	respectRobots bool    // 是否遵守 robots.txt
	maxDepth     int      // 最大爬取深度
	resume       bool     // 是否断点续爬

	// 服务相关标志
	port      int    // 服务端口
	bindAddr  string // 绑定地址

	// 打包相关标志
	packFormat string // 打包格式（zim 或 tar）
)

// NewRootCommand 创建根命令
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "shadow",
		Short: "ShadowWeb（影网）- 离线网站克隆工具",
		Long: `ShadowWeb（影网）是一个高性能的离线网站克隆工具，
基于 Headless Chrome 渲染页面，支持 JavaScript 剥离、资源本地化、
全文搜索索引和 ZIM 格式打包。`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				fmt.Fprintf(os.Stderr, "[ShadowWeb] 详细模式已启用\n")
			}
		},
	}

	// 注册全局持久标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用详细输出")
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 4, "并发工作线程数")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "./shadow-output", "输出目录")

	// 注册子命令
	rootCmd.AddCommand(newCloneCommand())
	rootCmd.AddCommand(newServeCommand())
	rootCmd.AddCommand(newPackCommand())
	rootCmd.AddCommand(newOpenCommand())
	rootCmd.AddCommand(newIndexCommand())

	return rootCmd
}

// newCloneCommand 创建 clone 子命令
func newCloneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <URL>",
		Short: "克隆网站到本地",
		Long: `使用 Headless Chrome 渲染并克隆目标网站，
支持双层 Worker 池（渲染 Worker + 资源 Worker）高效并行下载。`,
		Args: cobra.ExactArgs(1),
		RunE: runClone,
	}

	cmd.Flags().StringVar(&scopePrefix, "scope-prefix", "", "只爬取以此前缀开头的 URL")
	cmd.Flags().StringArrayVar(&excludeList, "exclude", nil, "排除包含指定模式的 URL（可多次使用）")
	cmd.Flags().Float64Var(&rateLimit, "rate-limit", 0, "每秒最大请求数（0 表示不限速）")
	cmd.Flags().StringVar(&cookieFile, "cookie-file", "", "从文件加载 Cookie（Netscape 格式）")
	cmd.Flags().BoolVar(&noJS, "no-js", true, "剥离 JavaScript 以减小体积")
	cmd.Flags().BoolVar(&respectRobots, "respect-robots", true, "遵守 robots.txt 规则")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 10, "最大爬取深度")
	cmd.Flags().BoolVar(&resume, "resume", false, "从上次中断处继续爬取（读取 state.json）")

	return cmd
}

// newServeCommand 创建 serve 子命令
func newServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve [目录]",
		Short: "启动本地 HTTP 服务器浏览克隆结果",
		Long:  `在本地启动 HTTP 服务器，用于预览已克隆的网站内容。`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runServe,
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "服务器监听端口")
	cmd.Flags().StringVar(&bindAddr, "bind", "127.0.0.1", "绑定地址")

	return cmd
}

// newPackCommand 创建 pack 子命令
func newPackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack [目录]",
		Short: "将克隆结果打包为 ZIM 或归档文件",
		Long:  `将已克隆的网站内容打包为 ZIM 格式或其他归档格式，便于分发和离线阅读。`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPack,
	}

	cmd.Flags().StringVarP(&packFormat, "format", "f", "zim", "打包格式：zim, tar, zip")

	return cmd
}

// newOpenCommand 创建 open 子命令
func newOpenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <文件.zim>",
		Short: "打开 ZIM 文件并启动浏览服务器",
		Long:  `直接打开 ZIM 格式的离线包，启动本地服务器进行浏览。`,
		Args:  cobra.ExactArgs(1),
		RunE:  runOpen,
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "服务器监听端口")
	cmd.Flags().StringVar(&bindAddr, "bind", "127.0.0.1", "绑定地址")

	return cmd
}

// newIndexCommand 创建 index 子命令
func newIndexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [目录]",
		Short: "为克隆内容构建全文搜索索引",
		Long:  `使用 Bleve 搜索引擎为已克隆的 HTML 内容构建全文索引，支持中文内容。`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runIndex,
	}

	return cmd
}

// runClone 执行克隆逻辑
func runClone(cmd *cobra.Command, args []string) error {
	targetURL := strings.TrimSpace(args[0])
	if targetURL == "" {
		return fmt.Errorf("目标 URL 不能为空")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 打印配置信息
	fmt.Fprintf(cmd.OutOrStdout(), "=== ShadowWeb 克隆任务 ===\n")
	fmt.Fprintf(cmd.OutOrStdout(), "目标 URL: %s\n", targetURL)
	fmt.Fprintf(cmd.OutOrStdout(), "输出目录: %s\n", outputDir)
	fmt.Fprintf(cmd.OutOrStdout(), "并发数: %d\n", concurrency)
	if scopePrefix != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "作用域前缀: %s\n", scopePrefix)
	}
	if len(excludeList) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "排除模式: %v\n", excludeList)
	}
	if rateLimit > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "限速: %.1f 请求/秒\n", rateLimit)
	}
	if cookieFile != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Cookie 文件: %s\n", cookieFile)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "剥离 JS: %v\n", noJS)
	fmt.Fprintf(cmd.OutOrStdout(), "遵守 robots.txt: %v\n", respectRobots)
	fmt.Fprintf(cmd.OutOrStdout(), "最大深度: %d\n", maxDepth)
	fmt.Fprintf(cmd.OutOrStdout(), "断点续爬: %v\n", resume)
	fmt.Fprintf(cmd.OutOrStdout(), "========================\n")

	// TODO: 调用 clone 包执行实际爬取
	fmt.Fprintf(cmd.OutOrStdout(), "克隆任务已启动（功能开发中）...\n")
	return nil
}

// runServe 启动本地服务器
func runServe(cmd *cobra.Command, args []string) error {
	dir := outputDir
	if len(args) > 0 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("解析目录路径失败: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("无法访问目录 %s: %w", absDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("指定路径不是目录: %s", absDir)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "启动服务器: http://%s:%d\n", bindAddr, port)
	fmt.Fprintf(cmd.OutOrStdout(), "服务目录: %s\n", absDir)

	// TODO: 调用 viewer 包启动服务器
	fmt.Fprintf(cmd.OutOrStdout(), "服务器运行中（功能开发中）...\n")
	return nil
}

// runPack 执行打包逻辑
func runPack(cmd *cobra.Command, args []string) error {
	sourceDir := outputDir
	if len(args) > 0 {
		sourceDir = args[0]
	}

	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("解析目录路径失败: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "打包目录: %s\n", absDir)
	fmt.Fprintf(cmd.OutOrStdout(), "打包格式: %s\n", packFormat)

	// TODO: 调用 pack 包执行打包
	fmt.Fprintf(cmd.OutOrStdout(), "打包任务已启动（功能开发中）...\n")
	return nil
}

// runOpen 打开 ZIM 文件
func runOpen(cmd *cobra.Command, args []string) error {
	zimPath := args[0]

	absPath, err := filepath.Abs(zimPath)
	if err != nil {
		return fmt.Errorf("解析文件路径失败: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("无法访问文件 %s: %w", absPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("指定路径是目录而非文件: %s", absPath)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "打开 ZIM 文件: %s\n", absPath)
	fmt.Fprintf(cmd.OutOrStdout(), "启动服务器: http://%s:%d\n", bindAddr, port)

	// TODO: 调用 viewer 包启动 ZIM 阅读服务器
	fmt.Fprintf(cmd.OutOrStdout(), "ZIM 阅读服务器运行中（功能开发中）...\n")
	return nil
}

// runIndex 构建搜索索引
func runIndex(cmd *cobra.Command, args []string) error {
	sourceDir := outputDir
	if len(args) > 0 {
		sourceDir = args[0]
	}

	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("解析目录路径失败: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "索引目录: %s\n", absDir)

	// TODO: 调用 index 包构建索引
	fmt.Fprintf(cmd.OutOrStdout(), "索引构建任务已启动（功能开发中）...\n")
	return nil
}
