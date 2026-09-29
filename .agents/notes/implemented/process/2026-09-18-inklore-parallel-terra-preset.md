# Agent Note: InkLore专用子代理配置使用独立preset

Status: implemented

Task: INK-144
Scope: DSH开发工具配置，不包含R1业务实现

## Problem

用户希望用DSH子代理并行开发，并指定GPT 5.6 txxxx模型。现有对话委派默认继承父模型；仓库没有可复用的项目代理配置，直接改全局模型会影响其他项目。

## Decision

检查已安装DSH模块和模型登记后，确定目标为basui/gpt-5.6-terra。以内置PTC为基础建立inklore-parallel-terra专用preset，13个委派工具使用固定agentOptions，reasoningEffort=high、maxDepth=1、continuable后台模式。保留项目源并安装到DSH_HOME用户preset目录，不修改全局默认。

11个角色覆盖平台、七业务域、双端和只读审阅。主代理按依赖最多安排4个活跃子代理；文件Owner为协作规则，不能伪称目录沙箱。审阅者工具白名单禁止shell与写入。公共装配/迁移/依赖/看板统一整合。

## Alternatives considered

只在提示词写模型名无法改变真实模型路由，因此不用。修改共享PTC或全局default会影响其他项目，因此新建专用preset。项目目录不是该版本DSH自动扫描根，故不能只放本地配置就宣称已启用。已开始对话preset锁定，不能修改持久会话文件绕过；新对话选择专用preset。

## Consequences

配置源修改后需同步用户目录；用户主代理仍可用其他模型，但所有新子代理固定Terra。fork跨模型不保证KV缓存复用。上游可用性/成本由供应商决定，配置校验不能代替真实模型调用结果。未开始业务开发。

## Verification

node .dsh/verify-preset.mjs退出0：DSH原生scanRoot发现且无broken，Config验证13个唯一工具，Terra/high已登记，项目与安装文件一致，review白名单正确。只验证配置与登记，未发送计费模型请求。使用与边界见[子代理规范](../../../../.dsh/SUBAGENTS.md)。
