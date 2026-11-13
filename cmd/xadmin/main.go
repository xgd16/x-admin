// Command xadmin 是 x-admin 框架的配套工具链。
//
// 通过 go.mod 的 tool 指令注册后，可以直接在仓库根目录运行：
//
//	go tool xadmin              # 终端中打开交互式 TUI
//	go tool xadmin upgrade      # 文本命令
//	go tool xadmin doctor
//
// 非交互环境（管道/CI）或设置 XADMIN_NO_TUI=1 时，直接运行会输出文本帮助。
// TUI 使用 bubbletea/bubbles/lipgloss，其他命令不会因此发起额外网络请求。
package main

import (
	"os"

	"x-admin/tool/xadmin"
)

func main() {
	os.Exit(xadmin.Run(os.Args[1:]))
}
