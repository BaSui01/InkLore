package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeadCodeItem 记录一项旧无用代码或废弃资产的登记信息。
type DeadCodeItem struct {
	Name             string `json:"name"`
	Category         string `json:"category"` // package | migration | table | route
	PathOrSymbol     string `json:"path_or_symbol"`
	RetirementReason string `json:"retirement_reason"`
	R1Replacement    string `json:"r1_replacement"`
	Status           string `json:"status"` // scheduled_for_removal | deprecated_freeze
}

// DeadCodeRegistry 维护需要随切换窗口最终物理清理的旧无用代码清单。
var DeadCodeRegistry = []DeadCodeItem{
	// ── 1. 旧业务领域包 (internal/novel) ──
	{
		Name:             "Legacy Novel Domain",
		Category:         "package",
		PathOrSymbol:     "internal/novel/domain/",
		RetirementReason: "旧粗粒度混杂领域包，已由 R1 content、outline、worldview 三大独立上下文细化拆解",
		R1Replacement:    "internal/content/domain/, internal/outline/domain/, internal/worldview/domain/",
		Status:           "permanently_removed",
	},
	{
		Name:             "Legacy Novel Application",
		Category:         "package",
		PathOrSymbol:     "internal/novel/application/",
		RetirementReason: "旧用例编排逻辑已被 R1 严格的单向依赖与 Port/Adapter 模型取代",
		R1Replacement:    "internal/content/application/, internal/worldview/application/",
		Status:           "permanently_removed",
	},
	{
		Name:             "Legacy Novel Infrastructure",
		Category:         "package",
		PathOrSymbol:     "internal/novel/infrastructure/",
		RetirementReason: "旧 SQL 仓储缺少严格的 R1 租户组合主键与 UoW 事务支持",
		R1Replacement:    "internal/content/infrastructure/, internal/platform/uow/",
		Status:           "permanently_removed",
	},
	{
		Name:             "Legacy Novel Transport",
		Category:         "package",
		PathOrSymbol:     "internal/novel/transport/http/",
		RetirementReason: "旧 /v1/novels 等非标准 HTTP API 路由，未满足 OpenAPI 3.1 严格契约与错误规范",
		R1Replacement:    "internal/content/transport/http/, internal/outline/transport/http/",
		Status:           "permanently_removed",
	},

	// ── 2. 旧版数据库迁移 (migrations/000001..000018) ──
	{
		Name:             "Legacy Migrations 000001-000018",
		Category:         "migration",
		PathOrSymbol:     "migrations/000001_init.sql ~ migrations/000018_llm_task_policies.sql",
		RetirementReason: "旧演进式单体迁移已被全新的 R1 物理表架构与分域迁移彻底取代",
		R1Replacement:    "migrations/r1/000001_platform.sql ~ migrations/r1/000005_llm.sql 及 data/*.sql",
		Status:           "deprecated_freeze",
	},

	// ── 3. 明确废弃的 7 张旧数据库表 ──
	{
		Name:             "novel_embeddings",
		Category:         "table",
		PathOrSymbol:     "public.novel_embeddings",
		RetirementReason: "旧本地静态 1536 维向量存储与 R1 独立微内核向量嵌入方案不兼容",
		R1Replacement:    "inspiration_segments / worldview_entities 实时嵌入",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "novel_summaries",
		Category:         "table",
		PathOrSymbol:     "public.novel_summaries",
		RetirementReason: "扁平粗糙的小说摘要表，已统一并入大纲树节点",
		R1Replacement:    "outline_nodes (master/volume) 与 content_novels.summary",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "agent_mcp_tool_cache",
		Category:         "table",
		PathOrSymbol:     "public.agent_mcp_tool_cache",
		RetirementReason: "MCP 动态工具临时缓存属于进程级易失内存数据，不予跨版本持久化",
		R1Replacement:    "agent_mcp_servers 运行时自动发现与内存缓存",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "user_agent_preferences",
		Category:         "table",
		PathOrSymbol:     "public.user_agent_preferences",
		RetirementReason: "个人偏好设置已被平台统一成员角色与智能体全局定义替代",
		R1Replacement:    "platform_memberships 与 agent_definitions",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "role_agent_access",
		Category:         "table",
		PathOrSymbol:     "public.role_agent_access",
		RetirementReason: "细粒度旧角色表已由平台三角色规范（creator/operator/auditor）统摄",
		R1Replacement:    "platform_memberships.roles",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "role_model_access",
		Category:         "table",
		PathOrSymbol:     "public.role_model_access",
		RetirementReason: "模型调用权限已统合入 LLM 调度域的 Task Policy 与 Channel 策略",
		R1Replacement:    "llm_task_policies 与 llm_channels",
		Status:           "scheduled_for_removal",
	},
	{
		Name:             "artifact_documents",
		Category:         "table",
		PathOrSymbol:     "public.artifact_documents",
		RetirementReason: "旧版粗糙产物文档表已被正式的平台文件与产物管理取代",
		R1Replacement:    "platform_files 与 agent_artifacts",
		Status:           "scheduled_for_removal",
	},

	// ── 4. 旧废弃路由 ──
	{
		Name:             "Legacy HTTP Routes",
		Category:         "route",
		PathOrSymbol:     "/v1/novels/*, /v1/characters/*, /v1/chapters/*",
		RetirementReason: "旧 API 路由未通过 OpenAPI 3.1 机器校验，缺乏统一错误包与 CAS 乐观锁",
		R1Replacement:    "api/content.json, api/outline.json, api/worldview.json",
		Status:           "scheduled_for_removal",
	},
}

// DeadCodeReport 汇总旧代码隔离与清理合规性报告。
type DeadCodeReport struct {
	Passed          bool           `json:"passed"`
	TotalRegistered int            `json:"total_registered"`
	CleanPackages   []string       `json:"clean_packages"`
	Violations      []string       `json:"violations"`
	RegisteredItems []DeadCodeItem `json:"registered_items"`
}

// VerifyDeadCodeIsolation 静态扫描 R1 领域包，确保无任何 R1 领域反向依赖 legacy internal/novel 包。
func VerifyDeadCodeIsolation(repoRoot string) DeadCodeReport {
	report := DeadCodeReport{
		Passed:          true,
		TotalRegistered: len(DeadCodeRegistry),
		RegisteredItems: DeadCodeRegistry,
	}

	r1Domains := []string{"content", "outline", "worldview", "inspiration", "compliance", "agent", "llm"}

	for _, domain := range r1Domains {
		domainPath := filepath.Join(repoRoot, "internal", domain)
		if _, err := os.Stat(domainPath); err != nil {
			continue
		}

		clean := true
		_ = filepath.Walk(domainPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
				data, readErr := os.ReadFile(path)
				if readErr == nil {
					content := string(data)
					if strings.Contains(content, "InkLore/internal/novel") {
						rel, _ := filepath.Rel(repoRoot, path)
						report.Violations = append(report.Violations, fmt.Sprintf("R1 领域代码 %s 非法导入旧包 internal/novel", rel))
						report.Passed = false
						clean = false
					}
				}
			}
			return nil
		})

		if clean {
			report.CleanPackages = append(report.CleanPackages, domain)
		}
	}

	return report
}
