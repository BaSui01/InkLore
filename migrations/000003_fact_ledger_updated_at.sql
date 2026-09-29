-- 000003：fact_ledger 补齐 updated_at
--
-- 000001 建表时遗漏 updated_at，而领域模型 FactEntry 需要它。
-- 刻意不改动已应用的 000001/000002：迁移执行器以校验和保护历史脚本，
-- 改写已应用的迁移会导致各环境库结构不一致（见 internal/platform/database/migrate.go）。

ALTER TABLE fact_ledger ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
