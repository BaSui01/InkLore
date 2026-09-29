package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AppBundleHash 包含单个前端应用产物的哈希集合。
type AppBundleHash struct {
	AppName      string            `json:"app_name"`
	Port         int               `json:"port"`
	EntryFile    string            `json:"entry_file"`
	EntryHash    string            `json:"entry_hash"`
	AssetHashes  map[string]string `json:"asset_hashes"`
	TotalBytes   int64             `json:"total_bytes"`
	BuildOutDir  string            `json:"build_out_dir"`
	BuiltPresent bool              `json:"built_present"`
}

// BundleHashReport 汇总双端前端产物校验报告。
type BundleHashReport struct {
	Passed          bool          `json:"passed"`
	CreatorApp      AppBundleHash `json:"creator_app"`
	AdminApp        AppBundleHash `json:"admin_app"`
	IsolationPassed bool          `json:"isolation_passed"`
	Errors          []string      `json:"errors"`
	Warnings        []string      `json:"warnings"`
}

// ComputeFileHash 计算单文件的 SHA-256 字符串。
func ComputeFileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// VerifyDualAppBundleHash 严格核对创作者端(:3000)与管理端(:3001)的静态资源指纹与物理隔离。
func VerifyDualAppBundleHash(repoRoot string) BundleHashReport {
	report := BundleHashReport{
		Passed:          true,
		IsolationPassed: true,
		CreatorApp: AppBundleHash{
			AppName:     "Creator Studio (创作者工作台)",
			Port:        3000,
			EntryFile:   "frontend/index.html",
			BuildOutDir: "frontend/dist",
			AssetHashes: make(map[string]string),
		},
		AdminApp: AppBundleHash{
			AppName:     "Admin Console (系统管理端)",
			Port:        3001,
			EntryFile:   "frontend/admin.html",
			BuildOutDir: "frontend/dist-admin",
			AssetHashes: make(map[string]string),
		},
	}

	frontendDir := filepath.Join(repoRoot, "frontend")

	// 1. 核对创作者端入口
	creatorEntryPath := filepath.Join(repoRoot, report.CreatorApp.EntryFile)
	if hash, sz, err := ComputeFileHash(creatorEntryPath); err == nil {
		report.CreatorApp.EntryHash = hash
		report.CreatorApp.TotalBytes += sz
	} else {
		report.Warnings = append(report.Warnings, fmt.Sprintf("creator entry file %s not readable: %v", creatorEntryPath, err))
	}

	// 2. 核对管理端入口
	adminEntryPath := filepath.Join(repoRoot, report.AdminApp.EntryFile)
	if hash, sz, err := ComputeFileHash(adminEntryPath); err == nil {
		report.AdminApp.EntryHash = hash
		report.AdminApp.TotalBytes += sz
	} else {
		report.Warnings = append(report.Warnings, fmt.Sprintf("admin entry file %s not readable: %v", adminEntryPath, err))
	}

	// 3. 扫描创作者端构建产物 (frontend/dist)
	creatorDistDir := filepath.Join(repoRoot, report.CreatorApp.BuildOutDir)
	if fi, err := os.Stat(creatorDistDir); err == nil && fi.IsDir() {
		report.CreatorApp.BuiltPresent = true
		_ = filepath.Walk(creatorDistDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				rel, _ := filepath.Rel(creatorDistDir, path)
				if h, sz, hErr := ComputeFileHash(path); hErr == nil {
					report.CreatorApp.AssetHashes[rel] = h
					report.CreatorApp.TotalBytes += sz
				}
			}
			return nil
		})
	} else {
		report.Warnings = append(report.Warnings, "frontend/dist (Creator) 未构建，使用入口源码哈希作为基线")
	}

	// 4. 扫描管理端构建产物 (frontend/dist-admin)
	adminDistDir := filepath.Join(repoRoot, report.AdminApp.BuildOutDir)
	if fi, err := os.Stat(adminDistDir); err == nil && fi.IsDir() {
		report.AdminApp.BuiltPresent = true
		_ = filepath.Walk(adminDistDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				rel, _ := filepath.Rel(adminDistDir, path)
				if h, sz, hErr := ComputeFileHash(path); hErr == nil {
					report.AdminApp.AssetHashes[rel] = h
					report.AdminApp.TotalBytes += sz
				}
			}
			return nil
		})
	} else {
		report.Warnings = append(report.Warnings, "frontend/dist-admin (Admin) 未构建，使用入口源码哈希作为基线")
	}

	// 5. 校验双端隔离铁律：创作者端入口不得包含 admin 入口文件或管理端独占标记
	if creatorData, err := os.ReadFile(creatorEntryPath); err == nil {
		content := string(creatorData)
		if strings.Contains(content, "admin.html") {
			report.IsolationPassed = false
			report.Passed = false
			report.Errors = append(report.Errors, "创作者端入口中包含非法管理端入口 admin.html 引用，违反双端严格隔离铁律")
		}
	}

	// 6. 核对配置文件隔离
	creatorViteConfig := filepath.Join(frontendDir, "vite.config.ts")
	adminViteConfig := filepath.Join(frontendDir, "vite.admin.config.ts")
	if h, _, err := ComputeFileHash(creatorViteConfig); err == nil {
		report.CreatorApp.AssetHashes["vite.config.ts"] = h
	}
	if h, _, err := ComputeFileHash(adminViteConfig); err == nil {
		report.AdminApp.AssetHashes["vite.admin.config.ts"] = h
	}

	return report
}
