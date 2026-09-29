// Package r1 provides the frozen R1 migration stream for a fresh database.
// Legacy migrations are deliberately excluded from this stream.
package r1

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var embedded embed.FS

func FS() fs.FS { return embedded }

const Dir = "."
