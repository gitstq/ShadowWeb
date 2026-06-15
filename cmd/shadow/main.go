// ShadowWeb（影网）- 离线网站克隆工具
// 基于 Headless Chrome 的高性能网站克隆器，支持 JS 剥离、资源本地化、全文搜索和 ZIM 打包。
package main

import (
	"fmt"
	"os"

	"github.com/gitstq/ShadowWeb/pkg/cli"
)

func main() {
	rootCmd := cli.NewRootCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[错误] %v\n", err)
		os.Exit(1)
	}
}
