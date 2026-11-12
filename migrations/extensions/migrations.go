// Package extensions 提供迁移扩展流：在主流之后应用，并用独立历史表记录，
// 保证主流 schema_migrations 的既定记录不被改写。
package extensions

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var embedded embed.FS

func FS() fs.FS { return embedded }

const Dir = "."
