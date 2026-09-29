package authdomain

import (
	"strings"
)

const (
	AuthCenterAppCode = "ai_content_platform"

	AuthCenterPermissionWildcard = "ai_content_platform.*"

	PermConsoleAccess          = "ai_content_platform.console:access"
	PermWorkspaceAccess        = "ai_content_platform.workspace:access"
	PermGenerationExecute      = "ai_content_platform.generation:execute"
	PermPromptRead             = "ai_content_platform.prompt:read"
	PermPromptWrite            = "ai_content_platform.prompt:write"
	PermPromptOptimize         = "ai_content_platform.prompt:optimize"
	PermModelRead              = "ai_content_platform.model:read"
	PermModelWrite             = "ai_content_platform.model:write"
	PermChannelRead            = "ai_content_platform.channel:read"
	PermChannelWrite           = "ai_content_platform.channel:write"
	PermBusinessUserRead       = "ai_content_platform.business_user:read"
	PermBusinessUserWrite      = "ai_content_platform.business_user:write"
	PermQuotaRead              = "ai_content_platform.quota:read"
	PermQuotaWrite             = "ai_content_platform.quota:write"
	PermOrgRead                = "ai_content_platform.org:read"
	PermOrgWrite               = "ai_content_platform.org:write"
	PermStatsRead              = "ai_content_platform.stats:read"
	PermStatsExport            = "ai_content_platform.stats:export"
	PermAuditRead              = "ai_content_platform.audit:read"
	PermAuditExport            = "ai_content_platform.audit:export"
	PermStorageRead            = "ai_content_platform.storage:read"
	PermStorageWrite           = "ai_content_platform.storage:write"
	PermConfigRead             = "ai_content_platform.config:read"
	PermConfigWrite            = "ai_content_platform.config:write"
	PermExecutorRuntimeOperate = "ai_content_platform.executor_runtime:operate"
	PermSyncExecute            = "ai_content_platform.sync:execute"

	// 审计 / 统计 / 组织 / 用户 — 数据范围
	PermAuditReadAll          = "ai_content_platform.audit:read:all"
	PermAuditReadDept         = "ai_content_platform.audit:read:dept"
	PermAuditReadTeam         = "ai_content_platform.audit:read:team"
	PermStatsReadAll          = "ai_content_platform.stats:read:all"
	PermStatsReadDept         = "ai_content_platform.stats:read:dept"
	PermStatsReadTeam         = "ai_content_platform.stats:read:team"
	PermOrgWriteAll           = "ai_content_platform.org:write:all"
	PermOrgWriteDept          = "ai_content_platform.org:write:dept"
	PermBusinessUserReadAll   = "ai_content_platform.business_user:read:all"
	PermBusinessUserReadDept  = "ai_content_platform.business_user:read:dept"
	PermBusinessUserReadTeam  = "ai_content_platform.business_user:read:team"
	PermBusinessUserWriteAll  = "ai_content_platform.business_user:write:all"
	PermBusinessUserWriteDept = "ai_content_platform.business_user:write:dept"

	// 素材中心
	PermMediaAssetRead     = "ai_content_platform.media_asset:read"
	PermMediaAssetCreate   = "ai_content_platform.media_asset:create"
	PermMediaAssetUpdate   = "ai_content_platform.media_asset:update"
	PermMediaAssetDelete   = "ai_content_platform.media_asset:delete"
	PermMediaAssetImport   = "ai_content_platform.media_asset:import"
	PermMediaAssetReview   = "ai_content_platform.media_asset:review"
	PermMediaAssetExecute  = "ai_content_platform.media_asset:execute"
	PermMediaAssetReadAll  = "ai_content_platform.media_asset:read:all"
	PermMediaAssetReadDept = "ai_content_platform.media_asset:read:dept"
	PermMediaAssetReadTeam = "ai_content_platform.media_asset:read:team"

	// 无限画布
	PermStyleProjectRead     = "ai_content_platform.style_project:read"
	PermStyleProjectCreate   = "ai_content_platform.style_project:create"
	PermStyleProjectUpdate   = "ai_content_platform.style_project:update"
	PermStyleProjectDelete   = "ai_content_platform.style_project:delete"
	PermStyleProjectExecute  = "ai_content_platform.style_project:execute"
	PermStyleProjectDownload = "ai_content_platform.style_project:download"
	PermStyleProjectPublish  = "ai_content_platform.style_project:publish"
	PermStyleProjectRetry    = "ai_content_platform.style_project:retry"

	// 店铺目录 / 直播间店铺绑定
	PermStoreDirectoryRead   = "ai_content_platform.store_directory:read"
	PermStoreDirectoryManage = "ai_content_platform.store_directory:manage"

	// 款号档案
	PermStyleLibraryRead   = "ai_content_platform.style_library:read"
	PermStyleLibraryCreate = "ai_content_platform.style_library:create"
	PermStyleLibraryUpdate = "ai_content_platform.style_library:update"
	PermStyleLibraryDelete = "ai_content_platform.style_library:delete"
	PermStyleLibraryImport = "ai_content_platform.style_library:import"
	PermStyleLibrarySync   = "ai_content_platform.style_library:sync"

	// 工作流 / 生成任务
	PermWorkflowRead         = "ai_content_platform.workflow:read"
	PermWorkflowCreate       = "ai_content_platform.workflow:create"
	PermWorkflowDelete       = "ai_content_platform.workflow:delete"
	PermWorkflowDownload     = "ai_content_platform.workflow:download"
	PermWorkflowFavorite     = "ai_content_platform.workflow:favorite"
	PermGenerationTaskRead   = "ai_content_platform.generation_task:read"
	PermGenerationTaskCreate = "ai_content_platform.generation_task:create"

	// Agent / 对话 / Skill
	PermAgentRead            = "ai_content_platform.agent:read"
	PermAgentCreate          = "ai_content_platform.agent:create"
	PermAgentUpdate          = "ai_content_platform.agent:update"
	PermAgentDelete          = "ai_content_platform.agent:delete"
	PermAgentApprovalRead    = "ai_content_platform.agent_approval:read"
	PermAgentApprovalApprove = "ai_content_platform.agent_approval:approve"
	PermAgentApprovalReject  = "ai_content_platform.agent_approval:reject"
	PermAgentRunRead         = "ai_content_platform.agent_run:read"
	PermChatSessionRead      = "ai_content_platform.chat_session:read"
	PermChatSessionCreate    = "ai_content_platform.chat_session:create"
	PermChatSessionUpdate    = "ai_content_platform.chat_session:update"
	PermChatSessionDelete    = "ai_content_platform.chat_session:delete"
	PermChatSessionExport    = "ai_content_platform.chat_session:export"
	PermSkillRead            = "ai_content_platform.skill:read"
	PermSkillCreate          = "ai_content_platform.skill:create"
	PermSkillUpdate          = "ai_content_platform.skill:update"
	PermSkillDelete          = "ai_content_platform.skill:delete"
	PermSkillUpload          = "ai_content_platform.skill:upload"

	// 相册 / 市场 / MCP
	PermAlbumRead         = "ai_content_platform.album:read"
	PermAlbumCreate       = "ai_content_platform.album:create"
	PermAlbumUpdate       = "ai_content_platform.album:update"
	PermAlbumDelete       = "ai_content_platform.album:delete"
	PermMarketplaceRead   = "ai_content_platform.marketplace:read"
	PermMarketplaceCreate = "ai_content_platform.marketplace:create"
	PermMarketplaceUpdate = "ai_content_platform.marketplace:update"
	PermMarketplaceDelete = "ai_content_platform.marketplace:delete"
	PermMCPServerRead     = "ai_content_platform.mcp_server:read"
	PermMCPServerCreate   = "ai_content_platform.mcp_server:create"
	PermMCPServerUpdate   = "ai_content_platform.mcp_server:update"
	PermMCPServerDelete   = "ai_content_platform.mcp_server:delete"

	// 商品发布独立领地
	PermProductPublishRead    = "ai_content_platform.product_publish:read"
	PermProductPublishWrite   = "ai_content_platform.product_publish:write"
	PermProductPublishExecute = "ai_content_platform.product_publish:execute"
	PermProductPublishConfig  = "ai_content_platform.product_publish:config"

	// 租户 / 计费 / 物理销毁 / OpenAPI 凭证
	PermTenantWrite    = "ai_content_platform.tenant:write"
	PermBillingManage  = "ai_content_platform.billing:manage"
	PermSystemPurge    = "ai_content_platform.system:purge"
	PermAPITokenManage = "ai_content_platform.api_token:manage"

	// 防泄密导出与快速切流
	PermMediaAssetExport   = "ai_content_platform.media_asset:export"
	PermStyleProjectExport = "ai_content_platform.style_project:export"
	PermChannelToggle      = "ai_content_platform.channel:toggle"
	PermModelToggle        = "ai_content_platform.model:toggle"

	// 生图方案与工作流数据范围
	PermStyleProjectReadAll  = "ai_content_platform.style_project:read:all"
	PermStyleProjectReadDept = "ai_content_platform.style_project:read:dept"
	PermStyleProjectReadTeam = "ai_content_platform.style_project:read:team"
	PermWorkflowReadAll      = "ai_content_platform.workflow:read:all"
	PermWorkflowReadDept     = "ai_content_platform.workflow:read:dept"
	PermWorkflowReadTeam     = "ai_content_platform.workflow:read:team"

	// 高成本视频大模型生成
	PermVideoGenerateExecute = "ai_content_platform.generation_video:execute"

	// 数字人主播与模特专属资产
	PermCharacterRead  = "ai_content_platform.character:read"
	PermCharacterWrite = "ai_content_platform.character:write"
	PermCharacterBind  = "ai_content_platform.character:bind"

	// 电商发品终审复核
	PermProductPublishApprove = "ai_content_platform.product_publish:approve"

	// 提示词范围隔离
	PermPromptReadAll  = "ai_content_platform.prompt:read:all"
	PermPromptReadDept = "ai_content_platform.prompt:read:dept"
	PermPromptReadTeam = "ai_content_platform.prompt:read:team"

	// 算力队列与高并发特权
	PermGenerationBatch    = "ai_content_platform.generation:batch"
	PermGenerationPriority = "ai_content_platform.generation:priority"

	// 供应链款号成本价脱敏
	PermStyleLibraryCostRead = "ai_content_platform.style_library:cost:read"

	// 智能体高危工具门禁
	PermAgentToolExecute = "ai_content_platform.agent_tool:execute"

	// 小组店铺绑定归属
	PermStoreDirectoryBindTeam = "ai_content_platform.store_directory:bind:team"

	// 实战细分与风控
	PermMediaAssetRestore     = "ai_content_platform.media_asset:restore"
	PermCharacterFavorite     = "ai_content_platform.character:favorite"
	PermProductPublishDelist  = "ai_content_platform.product_publish:delist"
	PermStyleProjectTemplate  = "ai_content_platform.style_project:template"
	PermStyleLibrarySyncForce = "ai_content_platform.style_library:sync_force"
	PermAgentShare            = "ai_content_platform.agent:share"
)

// AuthCenterPermissionCodes 是认证中心可导入权限目录的内容平台真相源。
// 与 docs/commercialization/permissions/ai-content-platform-permission-template-import.json 对齐。
var AuthCenterPermissionCodes = []string{
	PermConsoleAccess,
	PermWorkspaceAccess,
	PermGenerationExecute,
	PermPromptRead,
	PermPromptWrite,
	PermPromptOptimize,
	PermModelRead,
	PermModelWrite,
	PermChannelRead,
	PermChannelWrite,
	PermBusinessUserRead,
	PermBusinessUserWrite,
	PermQuotaRead,
	PermQuotaWrite,
	PermOrgRead,
	PermOrgWrite,
	PermStatsRead,
	PermStatsExport,
	PermAuditRead,
	PermAuditExport,
	PermStorageRead,
	PermStorageWrite,
	PermConfigRead,
	PermConfigWrite,
	PermExecutorRuntimeOperate,
	PermSyncExecute,
	PermAuditReadAll,
	PermAuditReadDept,
	PermAuditReadTeam,
	PermStatsReadAll,
	PermStatsReadDept,
	PermStatsReadTeam,
	PermOrgWriteAll,
	PermOrgWriteDept,
	PermBusinessUserReadAll,
	PermBusinessUserReadDept,
	PermBusinessUserReadTeam,
	PermBusinessUserWriteAll,
	PermBusinessUserWriteDept,
	PermMediaAssetRead,
	PermMediaAssetCreate,
	PermMediaAssetUpdate,
	PermMediaAssetDelete,
	PermMediaAssetImport,
	PermMediaAssetReview,
	PermMediaAssetExecute,
	PermMediaAssetReadAll,
	PermMediaAssetReadDept,
	PermMediaAssetReadTeam,
	PermStyleProjectRead,
	PermStyleProjectCreate,
	PermStyleProjectUpdate,
	PermStyleProjectDelete,
	PermStyleProjectExecute,
	PermStyleProjectDownload,
	PermStyleProjectPublish,
	PermStyleProjectRetry,
	PermStoreDirectoryRead,
	PermStoreDirectoryManage,
	PermStyleLibraryRead,
	PermStyleLibraryCreate,
	PermStyleLibraryUpdate,
	PermStyleLibraryDelete,
	PermStyleLibraryImport,
	PermStyleLibrarySync,
	PermWorkflowRead,
	PermWorkflowCreate,
	PermWorkflowDelete,
	PermWorkflowDownload,
	PermWorkflowFavorite,
	PermGenerationTaskRead,
	PermGenerationTaskCreate,
	PermAgentRead,
	PermAgentCreate,
	PermAgentUpdate,
	PermAgentDelete,
	PermAgentApprovalRead,
	PermAgentApprovalApprove,
	PermAgentApprovalReject,
	PermAgentRunRead,
	PermChatSessionRead,
	PermChatSessionCreate,
	PermChatSessionUpdate,
	PermChatSessionDelete,
	PermChatSessionExport,
	PermSkillRead,
	PermSkillCreate,
	PermSkillUpdate,
	PermSkillDelete,
	PermSkillUpload,
	PermAlbumRead,
	PermAlbumCreate,
	PermAlbumUpdate,
	PermAlbumDelete,
	PermMarketplaceRead,
	PermMarketplaceCreate,
	PermMarketplaceUpdate,
	PermMarketplaceDelete,
	PermMCPServerRead,
	PermMCPServerCreate,
	PermMCPServerUpdate,
	PermMCPServerDelete,
	PermProductPublishRead,
	PermProductPublishWrite,
	PermProductPublishExecute,
	PermProductPublishConfig,
	PermTenantWrite,
	PermBillingManage,
	PermSystemPurge,
	PermAPITokenManage,
	PermMediaAssetExport,
	PermStyleProjectExport,
	PermChannelToggle,
	PermModelToggle,
	PermStyleProjectReadAll,
	PermStyleProjectReadDept,
	PermStyleProjectReadTeam,
	PermWorkflowReadAll,
	PermWorkflowReadDept,
	PermWorkflowReadTeam,
	PermVideoGenerateExecute,
	PermCharacterRead,
	PermCharacterWrite,
	PermCharacterBind,
	PermProductPublishApprove,
	PermPromptReadAll,
	PermPromptReadDept,
	PermPromptReadTeam,
	PermGenerationBatch,
	PermGenerationPriority,
	PermStyleLibraryCostRead,
	PermAgentToolExecute,
	PermStoreDirectoryBindTeam,
	PermMediaAssetRestore,
	PermCharacterFavorite,
	PermProductPublishDelist,
	PermStyleProjectTemplate,
	PermStyleLibrarySyncForce,
	PermAgentShare,
}

func HasAuthCenterPermission(user *User, permission string) bool {
	return PrincipalHasPermission(PrincipalFromUser(user), permission)
}

// AuthCenterPermissionAlternatives 返回「满足 required」可接受的权限码集合（含 required 自身）。
// 支持向后兼容旧版粗粒度权限码（如存量Token仍能访问新路由）。
func AuthCenterPermissionAlternatives(required string) []string {
	required = strings.TrimSpace(required)
	if required == "" {
		return nil
	}
	if !strings.HasPrefix(required, AuthCenterAppCode+".") {
		required = AuthCenterAppCode + "." + required
	}
	switch required {
	case PermProductPublishRead:
		return []string{PermProductPublishRead, PermMediaAssetRead}
	case PermProductPublishWrite:
		return []string{PermProductPublishWrite, PermStyleProjectPublish, PermMediaAssetUpdate}
	case PermProductPublishExecute:
		return []string{PermProductPublishExecute, PermStyleProjectPublish}
	case PermProductPublishConfig:
		return []string{PermProductPublishConfig, PermStyleProjectPublish, PermConfigWrite}
	case PermTenantWrite:
		return []string{PermTenantWrite, PermConfigWrite}
	case PermBillingManage:
		return []string{PermBillingManage, PermConfigWrite, PermQuotaWrite}
	case PermSystemPurge:
		return []string{PermSystemPurge, PermConfigWrite, PermStyleProjectDelete}
	case PermAPITokenManage:
		return []string{PermAPITokenManage, PermConfigWrite}
	case PermMediaAssetExport:
		return []string{PermMediaAssetExport, PermMediaAssetRead}
	case PermStyleProjectExport:
		return []string{PermStyleProjectExport, PermStyleProjectRead}
	case PermChannelToggle:
		return []string{PermChannelToggle, PermChannelWrite}
	case PermModelToggle:
		return []string{PermModelToggle, PermModelWrite}
	case PermVideoGenerateExecute:
		return []string{PermVideoGenerateExecute, PermGenerationExecute}
	case PermCharacterRead:
		return []string{PermCharacterRead, PermStyleProjectRead, PermMediaAssetRead}
	case PermCharacterWrite:
		return []string{PermCharacterWrite, PermStyleProjectUpdate, PermStyleProjectDelete, PermMediaAssetUpdate}
	case PermCharacterBind:
		return []string{PermCharacterBind, PermStoreDirectoryManage, PermStyleProjectUpdate}
	case PermProductPublishApprove:
		return []string{PermProductPublishApprove, PermProductPublishExecute, PermStyleProjectPublish}
	case PermStyleProjectReadAll:
		return []string{PermStyleProjectReadAll, PermStyleProjectRead}
	case PermStyleProjectReadDept:
		return []string{PermStyleProjectReadDept, PermStyleProjectReadAll, PermStyleProjectRead}
	case PermStyleProjectReadTeam:
		return []string{PermStyleProjectReadTeam, PermStyleProjectReadDept, PermStyleProjectReadAll, PermStyleProjectRead}
	case PermWorkflowReadAll:
		return []string{PermWorkflowReadAll, PermWorkflowRead}
	case PermWorkflowReadDept:
		return []string{PermWorkflowReadDept, PermWorkflowReadAll, PermWorkflowRead}
	case PermWorkflowReadTeam:
		return []string{PermWorkflowReadTeam, PermWorkflowReadDept, PermWorkflowReadAll, PermWorkflowRead}
	case PermPromptReadAll:
		return []string{PermPromptReadAll, PermPromptRead}
	case PermPromptReadDept:
		return []string{PermPromptReadDept, PermPromptReadAll, PermPromptRead}
	case PermPromptReadTeam:
		return []string{PermPromptReadTeam, PermPromptReadDept, PermPromptReadAll, PermPromptRead}
	case PermGenerationBatch:
		return []string{PermGenerationBatch, PermGenerationExecute}
	case PermGenerationPriority:
		return []string{PermGenerationPriority, PermGenerationExecute}
	case PermStyleLibraryCostRead:
		return []string{PermStyleLibraryCostRead, PermStyleLibraryRead}
	case PermAgentToolExecute:
		return []string{PermAgentToolExecute, PermAgentUpdate}
	case PermStoreDirectoryBindTeam:
		return []string{PermStoreDirectoryBindTeam, PermStoreDirectoryManage}
	case PermMediaAssetRestore:
		return []string{PermMediaAssetRestore, PermMediaAssetDelete, PermMediaAssetUpdate}
	case PermCharacterFavorite:
		return []string{PermCharacterFavorite, PermCharacterWrite, PermCharacterRead}
	case PermProductPublishDelist:
		return []string{PermProductPublishDelist, PermProductPublishExecute, PermProductPublishWrite}
	case PermStyleProjectTemplate:
		return []string{PermStyleProjectTemplate, PermStyleProjectUpdate}
	case PermStyleLibrarySyncForce:
		return []string{PermStyleLibrarySyncForce, PermStyleLibraryImport}
	case PermMediaAssetReview:
		return []string{PermMediaAssetReview, PermStyleProjectUpdate, PermStyleProjectPublish, PermGenerationExecute}
	case PermAgentShare:
		return []string{PermAgentShare, PermAgentUpdate}
	default:
		return []string{required}
	}
}

// NormalizePermissionCode 将包含应用前缀的长权限码转换为纯短码（如 ai_content_platform.product_publish:read -> product_publish:read）
func NormalizePermissionCode(code string) string {
	code = strings.TrimSpace(code)
	return strings.TrimPrefix(code, AuthCenterAppCode+".")
}

// SatisfiesAuthCenterPermission 判断用户是否满足 required。
func SatisfiesAuthCenterPermission(user *User, required string) bool {
	for _, alt := range AuthCenterPermissionAlternatives(required) {
		if HasAuthCenterPermission(user, alt) {
			return true
		}
	}
	return false
}

func HasAllAuthCenterPermissions(user *User, permissions ...string) bool {
	for _, permission := range permissions {
		if !SatisfiesAuthCenterPermission(user, permission) {
			return false
		}
	}
	return true
}

func HasAnyAuthCenterPermission(user *User, permissions ...string) bool {
	for _, permission := range permissions {
		if SatisfiesAuthCenterPermission(user, permission) {
			return true
		}
	}
	return false
}

func HasAuthCenterSuperRole(user *User) bool {
	return PrincipalHasAuthCenterRole(PrincipalFromUser(user), RoleSuperAdmin)
}

func AuthCenterRoleMatches(granted string, wanted string) bool {
	return normalizeAuthCenterRole(granted) == normalizeAuthCenterRole(wanted)
}

func normalizeAuthCenterRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "admin", "super_admin", "platform.super_admin":
		return RoleSuperAdmin
	case "owner", "tenant_admin", "tenant.admin", "enterprise_admin", "merchant_admin":
		return RoleTenantAdmin
	case "general_manager":
		return RoleGeneralManager
	case "department_manager", "manager":
		return RoleDepartmentManager
	case "team_leader":
		return RoleTeamLeader
	case "employee":
		return RoleEmployee
	default:
		return role
	}
}
