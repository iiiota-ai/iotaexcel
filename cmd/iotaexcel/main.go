// Package main 是 iotaexcel 命令行程序入口。
//
// 所有实际逻辑都委托给 internal/app，main 只负责把系统参数传入并用返回值退出进程。
package main

import (
	"os"

	"iotaexcel/internal/app"
)

// main 启动 CLI。
// os.Args[1:] 去掉可执行文件名，app.Run 返回的退出码会原样作为进程退出码。
func main() {
	os.Exit(app.Run(os.Args[1:]))
}
