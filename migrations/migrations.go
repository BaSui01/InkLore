// Package migrations 以 embed 方式提供数据库迁移脚本。
//
// 迁移文件命名：NNNNNN_描述.sql（数字前缀决定执行顺序）。
// 文件可包含可选的降级段，以上述标记行分隔；没有降级段的迁移不支持回滚。
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var embedded embed.FS

// FS 返回包含全部迁移脚本的文件系统。
func FS() fs.FS {
	return embedded
}

// Dir 是迁移脚本在 FS 中的目录（根目录）。
const Dir = "."
