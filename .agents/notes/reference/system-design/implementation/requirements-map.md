> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../../AGENTS.md)；实现以源码和本轮验证为准。

# R1需求至API追踪

> UI-only需求FR-05/25另见15/17/20。每个operation的x-use-case需引用01需求编号。

| FR | operationId |
| --- | --- |
| FR-01 | authCaptcha, authChangePassword, authForgotPassword, authLogin, authLogout, authMe, authRefresh, authRegister, authResetPassword, authSessionCSRF, authVerifyCaptcha |
| FR-02 | content_archiveNovel, content_copyNovel, content_createNovel, content_getNovel, content_listNovels, content_updateNovel |
| FR-03 | content_deleteChapter, content_deleteNovel, content_deleteVolume, content_restoreChapter, content_restoreNovel, content_restoreVolume, inspiration_delete_asset, inspiration_delete_card, inspiration_delete_style, inspiration_restore_asset, inspiration_restore_card, inspiration_restore_style, outline_restoreBeat, outline_restoreClue, outline_restoreNode, outline_restoreScene, platformDeleteFile, worldview_restoreCharacter, worldview_restoreLore, worldview_restoreRelation |
| FR-04 | content_createChapter, content_createVolume, content_getChapter, content_getOrganization, content_getVolume, content_listchapters, content_listvolumes, content_reorderOrganization, content_updateChapter, content_updateVolume |
| FR-05 | content_getBody, content_saveBody |
| FR-06 | content_diffVersions, content_getVersion, content_listVersions, content_rollbackChapter |
| FR-07 | content_confirmChapter |
| FR-08 | outline_createBeat, outline_createNode, outline_createScene, outline_deleteBeat, outline_deleteNode, outline_deleteScene, outline_getBeat, outline_getNode, outline_getScene, outline_listbeats, outline_listnodes, outline_listscenes, outline_rebindNode, outline_reorder, outline_updateBeat, outline_updateNode, outline_updateScene |
| FR-09 | outline_createClue, outline_deleteClue, outline_getClue, outline_listClueEvents, outline_listclues, outline_pacing, outline_transitionClue, outline_updateClue |
| FR-10 | worldview_createCharacter, worldview_createLore, worldview_createRelation, worldview_deleteCharacter, worldview_deleteLore, worldview_deleteRelation, worldview_getCharacter, worldview_getLore, worldview_getRelation, worldview_graph, worldview_listcharacters, worldview_listlore, worldview_listrelations, worldview_updateCharacter, worldview_updateLore, worldview_updateRelation |
| FR-11 | worldview_checkFacts, worldview_getWorldRevision, worldview_listFactEvents, worldview_listFacts, worldview_retractFact, worldview_supersedeFact |
| FR-12 | agent_answer_novel_guide, agent_create_run, agent_get_artifact, agent_get_novel_guide, agent_list_artifacts, agent_start_novel_guide |
| FR-13 | agent_create_run, inspiration_create_analysis, inspiration_create_asset, inspiration_get_analysis, inspiration_get_asset, inspiration_get_report, inspiration_list_analyses, inspiration_list_assets, inspiration_list_segments, inspiration_update_asset |
| FR-14 | inspiration_create_card, inspiration_create_style, inspiration_get_card, inspiration_get_style, inspiration_get_style_version, inspiration_list_adoptions, inspiration_list_cards, inspiration_list_style_versions, inspiration_list_styles, inspiration_record_adoption, inspiration_revoke_adoption, inspiration_set_card_state, inspiration_update_card, inspiration_update_style |
| FR-15 | agent_create_run, compliance_create_report, compliance_create_rewrite, compliance_create_rule, compliance_create_rule_set, compliance_delete_rule, compliance_get_report, compliance_get_rewrite, compliance_get_rule_set, compliance_ignore_issue, compliance_list_issue_decisions, compliance_list_issues, compliance_list_reports, compliance_list_reviews, compliance_list_rewrites, compliance_list_rule_sets, compliance_list_rules, compliance_publish_rule_set, compliance_reopen_issue, compliance_scan, compliance_update_rule |
| FR-16 | agent_catalog_definitions, agent_catalog_task_policies, agent_catalog_workflows, agent_create_run, agent_get_run, agent_list_runs |
| FR-17 | agent_answer_run_question, agent_approve_approval, agent_cancel_run, agent_get_approval, agent_get_run_question, agent_list_approvals, agent_reject_approval, agent_subscribe_events, authWSTicket, platformEventsWebSocket |
| FR-18 | content_cancelJob, content_commitImport, content_createImport, content_getJob, content_listJobs, content_previewImport, platformGetFile, platformUploadFile |
| FR-19 | compliance_create_review, content_createExport, platformDownloadFile |
| FR-20 | content_search, content_statistics |
| FR-21 | llm_create_channel, llm_create_key, llm_create_model, llm_create_provider, llm_delete_channel, llm_delete_key, llm_delete_model, llm_delete_provider, llm_fetch_upstream_models, llm_get_channel, llm_get_key, llm_get_model, llm_get_provider, llm_list_channels, llm_list_keys, llm_list_models, llm_list_providers, llm_update_channel, llm_update_key, llm_update_model, llm_update_provider, platformListSecretReferences |
| FR-22 | llm_configure_budget, llm_create_policy, llm_create_pricing, llm_delete_policy, llm_get_budget, llm_get_policy, llm_get_pricing, llm_get_usage, llm_list_policies, llm_list_pricing, llm_list_usage, llm_publish_policy |
| FR-23 | agent_create_definition, agent_create_diagnostic, agent_create_mcp_server, agent_create_tool, agent_create_workflow, agent_delete_definition, agent_delete_mcp_server, agent_delete_tool, agent_delete_workflow, agent_get_definition, agent_get_diagnostic, agent_get_mcp_server, agent_get_tool, agent_get_workflow, agent_list_definitions, agent_list_mcp_servers, agent_list_tools, agent_list_workflows, agent_update_definition, agent_update_mcp_server, agent_update_tool, agent_update_workflow |
| FR-24 | agent_get_audit_run, agent_list_audit_runs, auth_switchTenant, platformCreateMember, platformExportAudit, platformGetMember, platformListAudit, platformListMembers, platformListProjectLogs, platformResetPassword, platformRevokeSessions, platformStatus, platformUpdateMember, platform_listTenants |
| FR-25 | UI控制器验收见15/17/20 |
