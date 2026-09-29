# Agent Note: Admin 渠道与 API Key 分离管理

Status: proposed

Task: INK-237
Scope: frontend/apps/admin/src/routes/ChannelsRoute.tsx、ChannelModal.tsx、ChannelKeysModal.tsx 与 LLM Admin API

## Problem

渠道删除返回 409 `llm.resource_in_use` 时，Admin 缺少对应双语词条而显示内部错误；渠道编辑还会把已存在的 SecretRef 再次 POST 成新 Key，导致重复绑定。后端已有 Key CRUD API，但此前没有独立 Key 管理界面。

## Proposal

让渠道 CRUD 只维护渠道字段。独立 Key 管理器完整接入既有 Key create/update/delete 操作；渠道列表展示 Key 数量并提供进入管理器的入口。存在 Key 绑定时禁用渠道删除，解除引用后再删除；后端 Reservation 等运行态冲突在对话框内显示安全、双语错误。

## Alternatives considered

- 删除渠道时级联删除所有 Key：会绕过后端 `llm.resource_in_use` 边界，且可能影响仍在使用的调度凭据。
- 仅翻译 409 错误码：能消除错误的内部错误提示，但仍无法管理导致阻塞的关联 Key，也无法阻止编辑时重复创建。

## Acceptance criteria

- 渠道 create/read/update/delete 与 Key create/read/update/delete 均经真实 Admin API。
- 编辑渠道不创建、更新或隐式删除 Key；Key 修改必须显式走 Key CRUD。
- 绑定 Key 时渠道删除有清晰阻止原因；活跃 Reservation 阻止删除 Key 时保留数据并显示 `llm.resource_in_use`。
- 未绑定 Key 的渠道可依 revision CAS 删除；修改期间发生版本竞争显示刷新重试提示。

## Risks

删除 Key 仍受后端活跃 Reservation 保护；渠道和多个 Key 的创建不是跨资源原子事务。本轮不级联删除、不修改现有渠道或密钥数据。需用隔离记录完成真实 Admin CRUD 行为验收。
