// Package version 提供应用的版本号、构建信息和 User-Agent 标识
//
// 运行时版本号来源优先级：
// 1. APP_VERSION 环境变量（运行时覆盖）
// 2. -ldflags 注入的 Version
// 3. VERSION 文件（编译时嵌入，推荐在 CI/CD 中自动更新）
//
// 打 tag 时同步更新 VERSION 文件：
//
//	echo "1.1.0" > backend/internal/platform/version/VERSION
//
// 或在 CI 中自动注入。
package version

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"
)

//go:embed VERSION
var embeddedVersion string

var (
	// Version 当前版本号，可在编译时通过 -ldflags 注入。
	Version = "dev"

	// GitCommit Git commit hash（短格式，例如：a1b2c3d）
	GitCommit = "unknown"

	// GitBranch Git 分支名（例如：master, dev）
	GitBranch = "unknown"

	// BuildTime 编译时间（ISO 8601 格式）
	BuildTime = "unknown"

	// GoVersion Go 编译器版本
	GoVersion = runtime.Version()

	// UserAgent 发送给上游 HTTP 请求的标识
	UserAgent string
)

func init() {
	// 引导期直读 env 的合理豁免：本 init 先于 internal/config 加载执行，
	// 且 platform 层不得反向依赖 config；APP_VERSION 语义等同 ldflags 注入。
	v := strings.TrimSpace(os.Getenv("APP_VERSION"))
	if v != "" {
		Version = v
	}
	if v == "" {
		if buildVersion := strings.TrimSpace(Version); buildVersion != "" && buildVersion != "dev" {
			v = buildVersion
		}
	}
	if v == "" {
		if embedded := strings.TrimSpace(embeddedVersion); embedded != "" {
			v = embedded
		}
	}
	if v == "" {
		v = strings.TrimSpace(Version)
	}

	UserAgent = fmt.Sprintf("ai-content-platform/%s", v)
}

// SetVersion 运行时覆盖版本号（供测试或特殊场景使用）
func SetVersion(v string) {
	v = strings.TrimSpace(v)
	if v != "" {
		Version = v
		UserAgent = fmt.Sprintf("ai-content-platform/%s", Version)
	}
}

// String 返回完整 User-Agent 字符串
func String() string {
	return UserAgent
}

// Info 版本信息结构体
type Info struct {
	Version   string `json:"version"`    // 版本号
	GitCommit string `json:"git_commit"` // Git commit hash
	GitBranch string `json:"git_branch"` // Git 分支
	BuildTime string `json:"build_time"` // 编译时间
	GoVersion string `json:"go_version"` // Go 版本
	OS        string `json:"os"`         // 操作系统
	Arch      string `json:"arch"`       // 架构
}
