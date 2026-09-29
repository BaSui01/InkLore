-- 000008：agent_runs.input_snapshot 补默认值（INK-045 探针发现）
--
-- 背景：000001 把 input_snapshot 定为 JSONB NOT NULL 且无默认值，但 agentrun 的
-- CreateRun 走的是 input_json 列、并不写 input_snapshot，导致该路径 100% 违反
-- not-null 约束（探针实测 SQLSTATE 23502）。recorder.go 路径会显式写该列，
-- 因此只补默认值而不放宽 NOT NULL。
--
-- 注：000007 已应用且受校验和保护，故以追加迁移修正。

ALTER TABLE agent_runs ALTER COLUMN input_snapshot SET DEFAULT '{}'::jsonb;