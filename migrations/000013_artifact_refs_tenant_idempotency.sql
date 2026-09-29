-- 000013：artifact_refs 幂等键纳入租户（INK-053）
--
-- 背景：000007 建的部分唯一索引只按 idempotency_key，导致两个租户使用相同幂等键时
-- 会命中同一行（跨租户共享幂等空间，甚至互相覆盖 attempt_count 与状态）。
--
-- 本迁移把索引重建为 (tenant_id, idempotency_key) 且保留同谓词 —— 与
-- ArtifactRefRepository.Begin 的 ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key <> ''
-- 必须逐字对应，否则 PostgreSQL 会报 no unique or exclusion constraint。

DROP INDEX IF EXISTS idx_artifact_refs_idempotency;
CREATE UNIQUE INDEX idx_artifact_refs_idempotency
    ON artifact_refs (tenant_id, idempotency_key)
    WHERE idempotency_key <> '';
