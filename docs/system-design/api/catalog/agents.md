# agents API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| POST /v1/agents/approvals/{approval_id}/approve | agent_approve_approval | FR-17 agent.approve_approval | [agent-runs.json](../agent-runs.json) |
| POST /v1/agents/approvals/{approval_id}/reject | agent_reject_approval | FR-17 agent.reject_approval | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/approvals/{approval_id} | agent_get_approval | FR-17 agent.get_approval | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/approvals | agent_list_approvals | FR-17 agent.list_approvals | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/artifacts/{artifact_id} | agent_get_artifact | FR-12 agent.get_artifact | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/audit/runs/{run_id} | agent_get_audit_run | FR-24 agent.get_audit_run | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/audit/runs | agent_list_audit_runs | FR-24 agent.list_audit_runs | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/catalog/definitions | agent_catalog_definitions | FR-16 agent.catalog_definitions | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/catalog/task-policies | agent_catalog_task_policies | FR-16 agent.catalog_task_policies | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/catalog/workflows | agent_catalog_workflows | FR-16 agent.catalog_workflows | [agent-runs.json](../agent-runs.json) |
| DELETE /v1/agents/definitions/{definition_id} | agent_delete_definition | FR-23 agent.delete_definition | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/definitions/{definition_id} | agent_get_definition | FR-23 agent.get_definition | [agent-admin.json](../agent-admin.json) |
| PUT /v1/agents/definitions/{definition_id} | agent_update_definition | FR-23 agent.update_definition | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/definitions | agent_list_definitions | FR-23 agent.list_definitions | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/definitions | agent_create_definition | FR-23 agent.create_definition | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/diagnostics/{run_id} | agent_get_diagnostic | FR-23 agent.get_diagnostic | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/diagnostics | agent_create_diagnostic | FR-23 agent.create_diagnostic | [agent-admin.json](../agent-admin.json) |
| DELETE /v1/agents/mcp-servers/{mcp_server_id} | agent_delete_mcp_server | FR-23 agent.delete_mcp_server | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/mcp-servers/{mcp_server_id} | agent_get_mcp_server | FR-23 agent.get_mcp_server | [agent-admin.json](../agent-admin.json) |
| PUT /v1/agents/mcp-servers/{mcp_server_id} | agent_update_mcp_server | FR-23 agent.update_mcp_server | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/mcp-servers | agent_list_mcp_servers | FR-23 agent.list_mcp_servers | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/mcp-servers | agent_create_mcp_server | FR-23 agent.create_mcp_server | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/novel-guide/{session_id}/answers | agent_answer_novel_guide | FR-12 agent.answer_novel_guide | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/novel-guide/{session_id} | agent_get_novel_guide | FR-12 agent.get_novel_guide | [agent-runs.json](../agent-runs.json) |
| POST /v1/agents/novel-guide | agent_start_novel_guide | FR-12 agent.start_novel_guide | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/runs/{run_id}/artifacts | agent_list_artifacts | FR-12 agent.list_artifacts | [agent-runs.json](../agent-runs.json) |
| POST /v1/agents/runs/{run_id}/cancel | agent_cancel_run | FR-17 agent.cancel_run | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/runs/{run_id}/events | agent_subscribe_events | FR-17 agent.subscribe_events | [agent-runs.json](../agent-runs.json) |
| POST /v1/agents/runs/{run_id}/questions/{question_id}/answer | agent_answer_run_question | FR-17 agent.answer_run_question | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/runs/{run_id}/questions/{question_id} | agent_get_run_question | FR-17 agent.get_run_question | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/runs/{run_id} | agent_get_run | FR-16 agent.get_run | [agent-runs.json](../agent-runs.json) |
| GET /v1/agents/runs | agent_list_runs | FR-16 agent.list_runs | [agent-runs.json](../agent-runs.json) |
| POST /v1/agents/runs | agent_create_run | FR-12 FR-13 FR-15 FR-16 agent.create_run | [agent-runs.json](../agent-runs.json) |
| DELETE /v1/agents/tools/{tool_id} | agent_delete_tool | FR-23 agent.delete_tool | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/tools/{tool_id} | agent_get_tool | FR-23 agent.get_tool | [agent-admin.json](../agent-admin.json) |
| PUT /v1/agents/tools/{tool_id} | agent_update_tool | FR-23 agent.update_tool | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/tools | agent_list_tools | FR-23 agent.list_tools | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/tools | agent_create_tool | FR-23 agent.create_tool | [agent-admin.json](../agent-admin.json) |
| DELETE /v1/agents/workflows/{workflow_id} | agent_delete_workflow | FR-23 agent.delete_workflow | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/workflows/{workflow_id} | agent_get_workflow | FR-23 agent.get_workflow | [agent-admin.json](../agent-admin.json) |
| PUT /v1/agents/workflows/{workflow_id} | agent_update_workflow | FR-23 agent.update_workflow | [agent-admin.json](../agent-admin.json) |
| GET /v1/agents/workflows | agent_list_workflows | FR-23 agent.list_workflows | [agent-admin.json](../agent-admin.json) |
| POST /v1/agents/workflows | agent_create_workflow | FR-23 agent.create_workflow | [agent-admin.json](../agent-admin.json) |
