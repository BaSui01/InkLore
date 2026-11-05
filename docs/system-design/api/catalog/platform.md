# platform API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| GET /v1/platform/audit-events/export | platformExportAudit | FR-24 platform.audit_export | [platform.json](../platform.json) |
| GET /v1/platform/audit-events | platformListAudit | FR-24 platform.audit | [platform.json](../platform.json) |
| GET /v1/platform/events | platformEventsWebSocket | FR-17 platform.events | [events.json](../events.json) |
| GET /v1/platform/files/{file_id}/download | platformDownloadFile | FR-19 platform.download | [platform.json](../platform.json) |
| DELETE /v1/platform/files/{file_id} | platformDeleteFile | FR-03 platform.delete_file | [platform.json](../platform.json) |
| GET /v1/platform/files/{file_id} | platformGetFile | FR-18 platform.file | [platform.json](../platform.json) |
| POST /v1/platform/files | platformUploadFile | FR-18 platform.upload | [platform.json](../platform.json) |
| POST /v1/platform/mail-settings/test | platformTestMailSettings | FR-24 platform.mail | [platform-mail.json](../platform-mail.json) |
| GET /v1/platform/mail-settings | platformGetMailSettings | FR-24 platform.mail | [platform-mail.json](../platform-mail.json) |
| PUT /v1/platform/mail-settings | platformUpdateMailSettings | FR-24 platform.mail | [platform-mail.json](../platform-mail.json) |
| POST /v1/platform/members/{member_id}/reset-password | platformResetPassword | FR-24 platform.reset_password | [platform.json](../platform.json) |
| POST /v1/platform/members/{member_id}/revoke-sessions | platformRevokeSessions | FR-24 platform.revoke_sessions | [platform.json](../platform.json) |
| GET /v1/platform/members/{member_id} | platformGetMember | FR-24 platform.member | [platform.json](../platform.json) |
| PUT /v1/platform/members/{member_id} | platformUpdateMember | FR-24 platform.update_member | [platform.json](../platform.json) |
| GET /v1/platform/members | platformListMembers | FR-24 platform.members | [platform.json](../platform.json) |
| POST /v1/platform/members | platformCreateMember | FR-24 platform.create_member | [platform.json](../platform.json) |
| GET /v1/platform/project-logs | platformListProjectLogs | FR-24 platform.project_logs | [platform-project-logs.json](../platform-project-logs.json) |
| GET /v1/platform/secret-references | platformListSecretReferences | FR-21 platform.secret_references | [platform.json](../platform.json) |
| GET /v1/platform/status | platformStatus | FR-24 platform.status | [platform.json](../platform.json) |
| GET /v1/platform/tenants | platform_listTenants | FR-24 platform.list_tenants | [platform.json](../platform.json) |
