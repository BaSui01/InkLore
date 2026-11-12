// Command archguard 校验 AGENTS.md 定义的架构红线中可自动化的部分。
//
// 校验项：
//
//	G1 跨域 infrastructure 直接导入（业务域 A 的文件导入业务域 B 的 infrastructure）
//	G2 platform 基座反向依赖业务域
//	G3 手写代码文件超过 500KiB 上限
//	G5 Go 源文件无法解析
//
// 附加项（不判定失败）：运行路径可达性报告，列出不在 cmd/server 依赖图中的包。
//
// 用法：
//
//	go run ./cmd/archguard                 # 校验，有违规时返回非零退出码
//	go run ./cmd/archguard -reachability   # 额外输出运行路径可达性报告
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	modulePath   = "InkLore"
	maxFileBytes = 500 * 1024
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "bin": true, "tmp": true,
	"dist": true, "dist-admin": true, "vendor": true, "coverage": true,
	".idea": true, ".vscode": true,
}

var sizeCheckedExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".css": true, ".sql": true,
}

// 新增或调整业务域时同步登记，避免跨域护栏漏掉真实边界。
var businessDomains = map[string]bool{
	"content":     true,
	"outline":     true,
	"worldview":   true,
	"inspiration": true,
	"compliance":  true,
	"agent":       true,
	"llm":         true,
}

type violation struct {
	Rule   string
	File   string
	Detail string
}

type ruleMeta struct {
	code string
	name string
}

var allRules = []ruleMeta{
	{"G1", "跨域 infrastructure 直接导入"},
	{"G2", "platform 反向依赖业务域"},
	{"G3", "单文件体积不超过 500KiB"},
	{"G5", "Go 源文件可解析"},
}

func main() {
	showReachability := flag.Bool("reachability", false, "输出运行路径可达性报告")
	flag.Parse()

	root, err := moduleRoot()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "archguard: 定位模块根目录失败: %v\n", err)
		os.Exit(2)
	}

	files, err := collectFiles(root)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "archguard: 遍历工作区失败: %v\n", err)
		os.Exit(2)
	}

	violations := make([]violation, 0)
	violations = append(violations, checkFileSizes(root, files)...)
	violations = append(violations, checkImports(root, files)...)

	report(violations)
	if *showReachability {
		printReachability(root)
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未能从当前目录向上找到 go.mod")
		}
		dir = parent
	}
}

func collectFiles(root string) ([]string, error) {
	out := make([]string, 0, 512)
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, walkErr
}

func checkFileSizes(root string, files []string) []violation {
	vs := make([]violation, 0)
	for _, f := range files {
		if !sizeCheckedExts[strings.ToLower(filepath.Ext(f))] {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		if info.Size() > maxFileBytes {
			vs = append(vs, violation{
				Rule:   "G3",
				File:   f,
				Detail: fmt.Sprintf("体积 %.1fKiB 超过 500KiB 上限，需垂直或水平拆分", float64(info.Size())/1024),
			})
		}
	}
	return vs
}

func checkImports(root string, files []string) []violation {
	fset := token.NewFileSet()
	vs := make([]violation, 0)
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(f))
		astFile, err := parser.ParseFile(fset, abs, nil, parser.ImportsOnly)
		if err != nil {
			vs = append(vs, violation{Rule: "G5", File: f, Detail: "Go 语法解析失败: " + err.Error()})
			continue
		}
		owner := ownerDomain(f)
		for _, spec := range astFile.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				continue
			}
			prefix := modulePath + "/internal/"
			if !strings.HasPrefix(importPath, prefix) {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(importPath, prefix), "/")
			if len(parts) < 2 {
				continue
			}
			targetDomain := parts[0]
			targetIsInfrastructure := parts[1] == "infrastructure"

			// G1：业务域文件不得直接导入其它业务域的 infrastructure。
			// 组合根 internal/app 与入口 cmd 承担依赖注入职责，予以豁免。
			if businessDomains[owner] && targetIsInfrastructure &&
				owner != targetDomain && businessDomains[targetDomain] {
				vs = append(vs, violation{
					Rule: "G1",
					File: f,
					Detail: fmt.Sprintf(
						"域 %q 直接导入了域 %q 的 infrastructure（%s）；应定义 Port 并在 internal/app/adapters 装配",
						owner, targetDomain, importPath),
				})
			}

			// G2：platform 基座不得反向依赖任何业务域。
			if owner == "platform" && businessDomains[targetDomain] {
				vs = append(vs, violation{
					Rule: "G2",
					File: f,
					Detail: fmt.Sprintf(
						"platform 基座反向依赖业务域 %q（%s）；基座必须保持无业务依赖",
						targetDomain, importPath),
				})
			}
		}
	}
	return vs
}

func ownerDomain(relPath string) string {
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 && parts[0] == "internal" {
		return parts[1]
	}
	return ""
}

func report(vs []violation) {
	byRule := make(map[string][]violation)
	for _, v := range vs {
		byRule[v.Rule] = append(byRule[v.Rule], v)
	}

	fmt.Println("archguard —— AGENTS.md 架构红线校验")
	fmt.Println(strings.Repeat("-", 64))
	for _, r := range allRules {
		items := byRule[r.code]
		if len(items) == 0 {
			fmt.Printf("✔ %s %s\n", r.code, r.name)
			continue
		}
		fmt.Printf("✘ %s %s（%d 处违规）\n", r.code, r.name, len(items))
		for _, it := range items {
			fmt.Printf("    %s\n      %s\n", it.File, it.Detail)
		}
	}
	fmt.Println(strings.Repeat("-", 64))
	if len(vs) > 0 {
		fmt.Printf("结果：发现 %d 处违规，构建应被阻断\n", len(vs))
		return
	}
	fmt.Println("结果：全部通过")
}

func printReachability(root string) {
	all, err := goList(root, "./...")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "archguard: 运行路径报告失败: %v\n", err)
		return
	}
	cmds, err := goList(root, "./cmd/...")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "archguard: 运行路径报告失败: %v\n", err)
		return
	}
	set := make(map[string]bool, 64)
	for _, cmdPkg := range cmds {
		deps, depErr := goList(root, "-deps", cmdPkg)
		if depErr != nil {
			continue
		}
		for _, dep := range deps {
			set[dep] = true
		}
	}
	unreachable := make([]string, 0)
	for _, p := range all {
		if set[p] || strings.Contains(p, "/cmd/") {
			continue
		}
		unreachable = append(unreachable, strings.TrimPrefix(p, modulePath+"/"))
	}

	fmt.Println()
	fmt.Println("运行路径可达性（信息项，不判定失败）")
	fmt.Println(strings.Repeat("-", 64))
	fmt.Printf("非 cmd 包总数 %d ｜ 可从 ./cmd/... 到达 %d ｜ 不可达 %d\n",
		len(all), len(all)-len(unreachable), len(unreachable))
	if len(unreachable) == 0 {
		fmt.Println("✔ 所有包均在运行路径上")
		return
	}
	for _, p := range unreachable {
		fmt.Printf("  - %s\n", p)
	}
}

func goList(root string, args ...string) ([]string, error) {
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	pkgs := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			pkgs = append(pkgs, l)
		}
	}
	return pkgs, nil
}
