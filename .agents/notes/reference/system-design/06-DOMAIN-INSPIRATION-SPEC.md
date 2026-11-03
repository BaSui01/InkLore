> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 灵感与拆书域 R1 冻结设计

状态：INK-141 实施目标，R1 冻结。唯一跨域约束见 [契约基线](00-CONTRACT-BASELINE.md)。详细用例、端口、事务与验收见 [详细设计](domains/06-DOMAIN-INSPIRATION-DETAIL-SPEC.md)，物理表见 [inspiration.sql](../../../../docs/system-design/data/inspiration.sql)，机读接口见 [查询](../../../../docs/system-design/api/inspiration-queries.json)、[命令](../../../../docs/system-design/api/inspiration-commands.json)、[文风与卡片命令](../../../../docs/system-design/api/inspiration-library.json)、[模型](../../../../docs/system-design/api/inspiration-schemas.json)。

## 1. 领域权威与闭环

inspiration 拥有外部素材 asset、确定性分片 segment、分析请求 analysis、拆书报告 report、版本化文风档案 style、灵感卡 card 与采纳引用 adoption。全部为建议资产，不拥有正文、大纲或事实。依赖只有 platform 技术能力与 llm 生成能力。Agent 通过自己声明的 InspirationPort 调用本域应用服务；本域绝不启动 Agent run。不存在从灵感域写 content/worldview/outline 的调用。

创作者闭环：上传 platform_files 或粘贴文本 → 创建素材 → 查看分片及版权来源 → 创建 pending 分析 → UI 创建 Agent run → 查看报告和证据 → 保存文风版本或灵感卡 → 作者在目标编辑器确认保存 → 记录采纳引用。撤销引用仅更改引用状态，不撤销正文或事实。对大纲的复用也由作者在 outline 编辑器提交，不提供“一键自动落事实”。

## 2. 用例与界面契约

| 用例 | 输入与操作 | 成功结果 | 失败处理 |
| --- | --- | --- | --- |
| INSP-01 导入素材 | POST assets；file_id 或 paste_text 二选一，title、作者、来源说明、rights_status 必填 | 201 ready asset，确定性分片一次落库 | 文件归属不符404；非UTF-8文本422；重复409附既有资源ID |
| INSP-02 拆书 | POST analyses kind=deconstruction，asset_id及revision、预算 | 201 pending；UI另发Agent任务 | 输入变化409；未就绪409；预算429 |
| INSP-03 文风提炼 | kind=style；样本至少1000码点 | 报告含句长、对话比例、感官倾向与证据 | 不宣称精确模仿某作者或保证输出风格 |
| INSP-04 灵感发散 | kind=ideas；提供目标scope与50..4000码点创意要求，可选asset | 报告含1..10条候选卡内容及证据 | 无素材时source_kind=original，不捏造引文 |
| INSP-05 管理文风 | POST styles；PUT styles/{style_id}追加版本 | 档案同ID revision增加，历史version_id不变 | CAS失败409；旧版本继续可复现 |
| INSP-06 保存卡片 | POST cards，手工或来自report candidate | 唯一内容指纹、类别与来源证据 | 重复409；作者可编辑已有卡而非复制 |
| INSP-07 采纳与撤销 | 作者先提交目标域保存；POST cards/{id}/adoptions附目标版本与hash；POST adoptions/{id}/revoke | 采纳引用active/revoked，有理由与审计 | 不读取目标域私表，不回滚目标事实 |
| INSP-08 删除与恢复 | DELETE asset/style/card；POST资源restore | 软删除30天，恢复同ID新revision | 引用保留最少溯源；30天后擦除原文 |

界面显示分析状态、实际预算使用、来源证据、版权声明、生成内容的建议性质。进度/SSE从Agent订阅，本域GET只读；WS只有失效通知。创作者只读自己的素材和文风，作品关联卡片还须通过novel scope。操作员不能读取原始素材。

## 3. 固定参数与状态

输入文件只接受UTF-8 text/plain，20MiB；粘贴1..200000码点且JSON≤2MiB。规范化仅CRLF/CR→LF，不trim、不改Unicode归一形；SHA256针对规范化UTF-8字节。版权字段包括作者0..120、source_url可空且≤2048、source_note1..2000、rights_status=owned/licensed/public_domain/unknown。unknown可以个人分析，禁止宣称获得转载许可；版权判断不自动完成。

每个asset最多200000码点，按段落边界尽量切成≤4000码点；无边界时硬切码点；分片无重叠，连续覆盖全文。证据定位是asset_id、asset_revision、segment_id、start_offset、end_offset、content_hash；offset为全文Unicode码点、左闭右开。excerpt最多500码点，必须等于对应源切片。

分析kind=deconstruction/style/ideas，状态pending→running→succeeded/failed/cancelled；pending也可cancelled。技术执行权由一个Agent run通过execution_key认领；终态不重开，重试业务创建新analysis。预算total_tokens=1000..120000，max_output_tokens=256..8000且≤total_tokens，max_calls=1..20；每次上下文加输出预留≤min(模型上限,60000)。每租户最多2个在途分析，每作者1个；限流由platform/llm admission统一执行。素材ready/deleted、档案active/deleted，卡片suggested/archived/deleted；是否采纳由引用计算，不以布尔覆盖历史。

## 4. R1验收门槛

必须完成可重放导入、证据定位、Agent异步分析、结构化拆书报告、文风版本选择、卡片去重、采纳与撤销引用、软删恢复和擦除闭环。所有写请求带Idempotency-Key，可变命令带expected_revision。校验用例包括非BMP字符定位、跨租户404、同键重放、并发版本冲突、模型超预算停止、运行取消后迟到结果不发布。具体步骤见详细设计。平台不把拆书结论或来源版权声明包装为保证。
