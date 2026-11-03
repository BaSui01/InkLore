// Package post_r1 provides the post-R1 extension migration stream.
// It applies only after the frozen R1 stream and tracks its own history
// so the frozen 17-entry R1 record stays untouched.
package post_r1

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var embedded embed.FS

func FS() fs.FS { return embedded }

const Dir = "."
