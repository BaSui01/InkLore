package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"InkLore/internal/agent/tool/domain"
	"InkLore/internal/platform/logger"
)

// WorkflowOrchestratorTool 智能工作流协同编排器。
//
// 遵循架构原则：工作流节点绝不在代码中硬编码。
// 工具根据用户的创作目标与作品上下文，由 LLM 动态自规划有向无环图（DAG）工作流节点，
// 自动调度预置子代理角色（novel-architect, editor-in-chief, novel-chapter-writer,
// consistency-reviewer, style-polisher）并分步执行，将每一步状态与产物实时呈现。
type WorkflowOrchestratorTool struct {
	generator       domain.TextGenerator
	recorder        domain.WorkflowNodeRecorder
	contextProvider domain.NovelContextProvider
}

// NewWorkflowOrchestratorTool 创建智能工作流编排工具。
func NewWorkflowOrchestratorTool(
	generator domain.TextGenerator,
	recorder domain.WorkflowNodeRecorder,
	contextProvider domain.NovelContextProvider,
) *WorkflowOrchestratorTool {
	return &WorkflowOrchestratorTool{
		generator:       generator,
		recorder:        recorder,
		contextProvider: contextProvider,
	}
}

func (t *WorkflowOrchestratorTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          "workflow_orchestrator",
		Name:        "智能工作流协同编排器",
		Description: "根据创作目标动态由大模型自主规划多智能体协同有向无环图（DAG），并调度预置子代理执行",
		Category:    "workflow",
		Parameters: []domain.ToolParam{
			{
				Name:        "task_goal",
				Type:        "string",
				Description: "具体的创作任务目标或要求（如：大纲规划、正文分步起草、设定冲突体检、去AI味润色）",
				Required:    true,
			},
			{
				Name:        "novel_id",
				Type:        "string",
				Description: "关联作品 ID（若提供，将自动提取世界观、角色状态与大纲等 Context Pack 真实上下文）",
				Required:    false,
			},
			{
				Name:        "chapter_id",
				Type:        "string",
				Description: "关联章节 ID（可选）",
				Required:    false,
			},
			{
				Name:        "context",
				Type:        "string",
				Description: "补充的背景说明、前情提要或风格指令",
				Required:    false,
			},
			{
				Name:        "custom_manifest",
				Type:        "string",
				Description: "自定义工作流拓扑结构（JSON 字符串，若提供则跳过自动规划直接执行）",
				Required:    false,
			},
		},
	}
}

func (t *WorkflowOrchestratorTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if t.generator == nil {
		return domain.ToolResult{
			ToolID:  "workflow_orchestrator",
			Success: false,
			Summary: "工作流编排器无法执行: 未装配大模型网关，无法驱动子代理规划与生成",
		}, nil
	}

	taskGoal, _ := input["task_goal"].(string)
	taskGoal = strings.TrimSpace(taskGoal)
	if taskGoal == "" {
		// 容错：如果调用方传入了 prompt 或 brief，自动识别
		if p, ok := input["prompt"].(string); ok && p != "" {
			taskGoal = p
		} else if b, ok := input["chapter_brief"].(string); ok && b != "" {
			taskGoal = b
		} else {
			return domain.ToolResult{
				ToolID:  "workflow_orchestrator",
				Success: false,
				Summary: "缺少必要的任务目标说明（task_goal）",
			}, nil
		}
	}

	novelID, _ := input["novel_id"].(string)
	chapterID, _ := input["chapter_id"].(string)
	extraContext, _ := input["context"].(string)

	// 1. 上下文配置与智能加载：自动载入小说真实的 Context Pack
	var loadedContextPack string
	if novelID != "" && t.contextProvider != nil {
		if packText, err := t.contextProvider.GetContextPack(ctx, novelID, chapterID); err == nil && packText != "" {
			loadedContextPack = packText
			logger.InfoContext(ctx, "工作流成功载入作品真实 Context Pack", "novel_id", novelID, "chapter_id", chapterID)
		}
	}

	// 2. 动态规划工作流 DAG（Dynamic Workflow Planning）
	var manifest domain.WorkflowManifest
	customManifestStr, _ := input["custom_manifest"].(string)

	if strings.TrimSpace(customManifestStr) != "" {
		if err := json.Unmarshal([]byte(customManifestStr), &manifest); err != nil {
			return domain.ToolResult{
				ToolID:  "workflow_orchestrator",
				Success: false,
				Summary: fmt.Sprintf("自定义工作流结构解析失败: %v", err),
			}, nil
		}
	} else {
		// 由大模型根据任务自规划 DAG
		planned, err := t.planWorkflow(ctx, taskGoal, loadedContextPack, extraContext)
		if err != nil {
			logger.WarnContext(ctx, "大模型自规划工作流失败，启用自适应默认编排", "error", err)
			planned = fallbackPlan(taskGoal)
		}
		manifest = *planned
	}

	if len(manifest.Nodes) == 0 {
		manifest = *fallbackPlan(taskGoal)
	}

	// 3. 将 Context Pack 与额外背景合并到执行上下文中
	execInput := make(map[string]any, len(input)+2)
	for k, v := range input {
		execInput[k] = v
	}
	if loadedContextPack != "" {
		execInput["novel_context_pack"] = loadedContextPack
	}
	if extraContext != "" {
		execInput["extra_context"] = extraContext
	}

	// 4. 调用通用的 WorkflowTool 执行引擎执行动态生成的 DAG
	workflowRunner := NewWorkflowTool(manifest, t.generator, t.recorder, nil)
	return workflowRunner.Execute(ctx, execInput)
}

// planWorkflow 调用大模型根据任务与上下文自主生成 DAG 拓扑
func (t *WorkflowOrchestratorTool) planWorkflow(
	ctx context.Context,
	taskGoal string,
	novelContextPack string,
	extraContext string,
) (*domain.WorkflowManifest, error) {
	prompt := fmt.Sprintf(`你是一个小说工程的多智能体协同工作流架构师。
你需要将作者的创作任务分解为一个结构合理、无循环依赖的有向无环图（DAG）工作流。

系统当前可调度的预置子代理角色（Subagents）：
1. novel-architect (架构创书 Agent): 负责世界法则推演、人物人设构建、核心伏笔设计
2. editor-in-chief (总编 Agent): 负责剧情大纲把控、卷章节拍切分、节奏张力诊断
3. novel-chapter-writer (章节创作 Agent): 负责依照大纲节拍起草小说正文、场景细节与对话描写
4. consistency-reviewer (连贯性审校 Agent): 负责对照世界观和角色设定排查逻辑违背、人设崩塌 (OOC)
5. style-polisher (文风润色 Agent): 负责重塑文学修辞、消除 AI 味陈词滥调、定稿输出

【用户创作目标】
%s

【作品背景与上下文包】
%s
%s

请根据任务的实际复杂度（避免冗余步骤），规划 2 到 4 个步骤的有向无环图（DAG）。
严格按如下 JSON 格式输出，不要包含任何 markdown 解释：
{
  "workflow_id": "dynamic_plan",
  "name": "工作流简明名称",
  "description": "整个流程的目标与逻辑概述",
  "nodes": [
    {
      "id": "node_1",
      "name": "步骤一中文名称",
      "type": "subagent",
      "subagent_name": "从上述5个子代理中选择最匹配的一个",
      "dependencies": [],
      "task_key": "outline",
      "description": "本步骤职责与目标",
      "task_prompt": "具体指示该子代理在此步骤需要完成的具体工作"
    }
  ]
}`, taskGoal, novelContextPack, extraContext)

	res, err := t.generator.GenerateText(ctx, domain.TextRequest{
		Prompt:             prompt,
		TaskKey:            "outline",
		DefaultTemperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	cleanJSON := cleanJSONFence(res.Content)
	var manifest domain.WorkflowManifest
	if err := json.Unmarshal([]byte(cleanJSON), &manifest); err != nil {
		return nil, fmt.Errorf("解析模型返回的工作流 JSON 失败: %w, 原文: %s", err, res.Content)
	}

	manifest.Category = "workflow"
	if manifest.WorkflowID == "" {
		manifest.WorkflowID = "dynamic_planned_workflow"
	}
	return &manifest, nil
}

func cleanJSONFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
	} else if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
	}
	if strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimSuffix(trimmed, "```")
	}
	return strings.TrimSpace(trimmed)
}

// fallbackPlan 当网络超时或无法调用模型规划时，提供自适应的保底规划
func fallbackPlan(taskGoal string) *domain.WorkflowManifest {
	return &domain.WorkflowManifest{
		WorkflowID:  "dynamic_planned_workflow",
		Name:        "智能创作协同流水线",
		Description: "动态编排：情境节拍规划 -> 正文分步起草 -> 审校与文风润色",
		Category:    "workflow",
		Nodes: []domain.WorkflowNodeDef{
			{
				ID:           "planning_step",
				Name:         "大纲与情境规划",
				Type:         "subagent",
				SubagentName: "editor-in-chief",
				Dependencies: []string{},
				TaskKey:      "outline",
				Description:  "明确核心冲突、场景节拍与推进目标",
				TaskPrompt:   fmt.Sprintf("围绕目标任务规划场景细纲与关键节拍：%s", taskGoal),
			},
			{
				ID:           "drafting_step",
				Name:         "正文创作起草",
				Type:         "subagent",
				SubagentName: "novel-chapter-writer",
				Dependencies: []string{"planning_step"},
				TaskKey:      "draft",
				Description:  "依据节拍规划起草正文草稿",
				TaskPrompt:   "基于前序规划的节拍，展开完整的情节描写与人物对白。",
			},
			{
				ID:           "refining_step",
				Name:         "合规审校与润色定稿",
				Type:         "subagent",
				SubagentName: "style-polisher",
				Dependencies: []string{"drafting_step"},
				TaskKey:      "deai",
				Description:  "排查人设与世界法则冲突，消除 AI 味，润色定稿",
				TaskPrompt:   "对起草正文进行审校，去除生硬修辞与陈词滥调，输出最终定稿正文。",
			},
		},
	}
}
