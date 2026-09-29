# compliance API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| GET /v1/compliance/reports/{report_id}/issues/{issue_id}/decisions | compliance_list_issue_decisions | FR-15 compliance.list_issue_decisions | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/reports/{report_id}/issues/{issue_id}/ignore | compliance_ignore_issue | FR-15 compliance.ignore_issue | [compliance-commands.json](../compliance-commands.json) |
| POST /v1/compliance/reports/{report_id}/issues/{issue_id}/reopen | compliance_reopen_issue | FR-15 compliance.reopen_issue | [compliance-commands.json](../compliance-commands.json) |
| GET /v1/compliance/reports/{report_id}/issues | compliance_list_issues | FR-15 compliance.list_issues | [compliance-queries.json](../compliance-queries.json) |
| GET /v1/compliance/reports/{report_id}/reviews | compliance_list_reviews | FR-15 compliance.list_reviews | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/reports/{report_id}/reviews | compliance_create_review | FR-19 compliance.create_review | [compliance-commands.json](../compliance-commands.json) |
| GET /v1/compliance/reports/{report_id} | compliance_get_report | FR-15 compliance.get_report | [compliance-queries.json](../compliance-queries.json) |
| GET /v1/compliance/reports | compliance_list_reports | FR-15 compliance.list_reports | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/reports | compliance_create_report | FR-15 compliance.create_report | [compliance-commands.json](../compliance-commands.json) |
| GET /v1/compliance/rewrites/{rewrite_id} | compliance_get_rewrite | FR-15 compliance.get_rewrite | [compliance-queries.json](../compliance-queries.json) |
| GET /v1/compliance/rewrites | compliance_list_rewrites | FR-15 compliance.list_rewrites | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/rewrites | compliance_create_rewrite | FR-15 compliance.create_rewrite | [compliance-commands.json](../compliance-commands.json) |
| POST /v1/compliance/rule-sets/{rule_set_id}/publish | compliance_publish_rule_set | FR-15 compliance.publish_rule_set | [compliance-commands.json](../compliance-commands.json) |
| DELETE /v1/compliance/rule-sets/{rule_set_id}/rules/{rule_id} | compliance_delete_rule | FR-15 compliance.delete_rule | [compliance-commands.json](../compliance-commands.json) |
| PUT /v1/compliance/rule-sets/{rule_set_id}/rules/{rule_id} | compliance_update_rule | FR-15 compliance.update_rule | [compliance-commands.json](../compliance-commands.json) |
| GET /v1/compliance/rule-sets/{rule_set_id}/rules | compliance_list_rules | FR-15 compliance.list_rules | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/rule-sets/{rule_set_id}/rules | compliance_create_rule | FR-15 compliance.create_rule | [compliance-commands.json](../compliance-commands.json) |
| GET /v1/compliance/rule-sets/{rule_set_id} | compliance_get_rule_set | FR-15 compliance.get_rule_set | [compliance-queries.json](../compliance-queries.json) |
| GET /v1/compliance/rule-sets | compliance_list_rule_sets | FR-15 compliance.list_rule_sets | [compliance-queries.json](../compliance-queries.json) |
| POST /v1/compliance/rule-sets | compliance_create_rule_set | FR-15 compliance.create_rule_set | [compliance-commands.json](../compliance-commands.json) |
| POST /v1/compliance/scans | compliance_scan | FR-15 compliance.scan | [compliance-commands.json](../compliance-commands.json) |
