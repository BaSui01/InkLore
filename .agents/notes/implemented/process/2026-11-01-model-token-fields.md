# Agent Note: 模型配置 Token 字段受控拆分

Status: implemented

Task: INK-250
Scope: frontend/apps/admin/src/routes/ModelConfigRoute.tsx 与 components/ModelTokenFields.tsx

## Problem

真实 archguard 报告17项50KB超限，其中一个为手写模型配置路由，另外16项为ADR0036规定的按域单文件生成契约。本次只处理无需改变设计规则的手写组件。

## Decision

沿 App models 视图到路由、创建/编辑初始化、输入状态和保存API链路读取后，将上下文/最大输出输入区域及原展示函数提取至现有 components 目录。子组件仅接收两个字符串与对应回调，翻译仍取同一 i18n 上下文；父路由保留所有状态、校验、默认值、类型切换、请求与 expected_revision。原JSX和展示内容只移动，不增加预设或改变保存行为。

## Alternatives considered

- 删除注释或压缩排版凑到50KB：不能改善职责边界，不采用。
- 放宽阈值或忽略文件：削弱门禁，禁止。
- 同时改变16个生成文件：ADR0036明确单文件布局，需要单独裁决，未实施。

## Consequences

路由49322字节，子组件4039字节，满足既有50KB限制。无新增Go包/依赖/接口，业务运行路径不变。生成器、冻结契约、规则和其忽略列表完全未改；生成契约超限仍保留。

## Verification

2026-11-01，dot独立dev来源快照：Go1.27.1 go build -buildvcs=false ./... 退出0；go run -buildvcs=false ./cmd/archguard -reachability 退出1，只剩16项生成契约大小违规，手写路由不再报错。前端与文档检查结果见本任务PR；不能把部分护栏改善称全绿或独立验收。未做登录后的浏览器输入/保存验证，未运行生产数据或付费模型。保持review/draft，不合并。
