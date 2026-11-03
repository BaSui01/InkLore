# ADR-0010：嵌入维度契约 —— 向量列维度固定，模型原生维度由上游按声明截断

- **状态**：Accepted
- **日期**：2026-09-17
- **决策者**：架构负责人（AI 协作）
- **关联任务**：INK-051
- **替代关系**：无

## 背景（Context）

M5 要求「同类事实语义检索 Top-3 命中率在抽样集上可用」，而 INK-017 只验证了**确定性嵌入桩**下的链路正确性。
要用真实嵌入模型完成评估，必须先回答一个契约问题：**向量维度由谁决定**。

现状约束（均为实测）：

- `novel_embeddings.embedding` 是 `vector(1536)`，并建有 HNSW 索引；
- pgvector **不允许**对无维度的 `vector` 列建 HNSW 索引（实测报错 `column does not have dimensions`），
  因此「把列改成无维度」会让相似度检索退化为全表扫描；
- 不同嵌入模型的原生维度不同：OpenAI `text-embedding-3-small` 为 1536，
  本地 TEI 部署的 `Qwen3-Embedding-4B` 为 2560；
- 若不解决，换模型只有两条路：改 schema（迁移不可改写 + 索引重建 + 历史向量全部重算），
  或者写入失败。

## 决策（Decision）

1. **向量列维度是 schema 的一部分，固定为 1536**（`novel_embeddings.embedding`）。换嵌入模型**不改 schema**。
2. **嵌入请求必须显式声明目标维度**：`llm/domain.EmbeddingRequest` 新增 `Dimensions`，
   出站载荷携带 OpenAI 兼容的 `dimensions` 字段，由上游按 MRL（Matryoshka）截断到该维度。
3. **维度由配置声明**：`EMBEDDING_DIMENSIONS`（默认 1536，与列维度一致）；取非默认值时启动即告警，
   提示「向量列维度需同步迁移」，避免把部署期错误留到写入时才暴露。
4. **维度不符必须显式失败**：返回向量长度与声明维度不一致时直接报错，
   **绝不静默截断、补零或丢弃**；写入失败时返回 `semantic_index_write_failed` 并带出 pgvector 原文。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| schema 跟随模型（每次换模型加迁移改列维度） | 每列维度都是模型原生维度，检索质量最高 | 换模型 = 改迁移 + 重建 HNSW + 历史向量全量重算；把部署期选择写进不可改写的迁移 | 迁移不可改写，代价被永久固化 |
| 列改为无维度 `vector` | 任意模型都能写入 | pgvector 拒绝对无维度列建 HNSW（实测），检索退化 | 违背「Top-3 召回可用」的目标 |
| 应用层自己截断到 1536 | 不依赖上游能力 | 需自行实现并维护 MRL 截断与重归一化；OpenAI / TEI 已原生支持 | 重复造轮子 |
| **显式声明维度 + 上游截断（采纳）** | 不改 schema；换模型只改配置；截断由上游负责 | 依赖上游支持 `dimensions`；质量随维度下降 | — |

## 后果（Consequences）

**正面**

- 换嵌入模型从「改 schema」降级为「改一个环境变量」，并可在同一套索引上横向比较不同模型的召回质量。
- 维度不一致在三个位置都会被挡住：上游响应校验、应用层校验、pgvector 写入；错误信息可定位到根因。

**负面 / 代价**

- 1536 维是 `Qwen3-Embedding-4B` 的 MRL 截断结果，检索质量低于其原生 2560 维 —— 这是「不改 schema」付出的代价。
- 若某上游不支持 `dimensions`（截断后仍返回原生维度），必须先在应用层截断或迁移列维度。

**需要长期承担的义务**

- 任何改变列维度的迁移都必须**同时**更新 `EMBEDDING_DIMENSIONS` 与本文档，并重建 HNSW 索引。
- 接入不支持 `dimensions` 的模型时，必须在应用层实现截断并新增 ADR 记录，而不是放宽校验。

## 验证方式（Verification）

1. TEI（`Qwen3-Embedding-4B`，`/info` 实测 `model_id=/data/models/Qwen3-Embedding-4B`）
   `POST /v1/embeddings {"dimensions":1536}` 返回 `data[0].embedding.Count = 1536`。
2. `POST /v1/novels/:novelID/embeddings/index` 在真实模型下写入 15 条向量，
   `select min(vector_dims(embedding)), max(vector_dims(embedding))` 均为 **1536**。
3. `EMBEDDING_DIMENSIONS=999` 时：启动日志出现「嵌入维度与内置默认值不一致」WARN；
   索引接口返回 `semantic_index_write_failed` 且消息含 `expected 1536 dimensions, not 999`。
4. 抽样评估：24 条查询 Top-3 命中 23 条（**95.8%**），记录见
   [semantic-recall-evaluation.md](../../../../docs/semantic-recall-evaluation.md)。
