// Command xadmin 是 x-admin 框架的配套工具链。
//
// 通过 go.mod 的 tool 指令注册后，可以直接在仓库根目录运行：
//
//	go tool xadmin upgrade check
//	go tool xadmin upgrade plan
//	go tool xadmin upgrade apply
//
// 工具只依赖标准库与仓库已经引入的 yaml 包，避免给框架增加额外依赖。
package main

import (
	"os"

	"x-admin/tool/xadmin"
)

func main() {
	os.Exit(xadmin.Run(os.Args[1:]))
}
