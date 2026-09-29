-- 000006：llm_usage_records.channel_id 允许为空
--
-- 背景：直连部署（未配置 LLM_CHANNELS）没有对应的 llm_channels 行，
-- 写入 channel_id=0 会违反 llm_usage_records_channel_id_fkey 外键，
-- 导致审计记录 100% 写入失败（探针实测，INK-018）。
-- 语义：NULL = 直连渠道（未经选路池命中具体渠道行）；非空 = 命中的 llm_channels.id。

ALTER TABLE llm_usage_records ALTER COLUMN channel_id DROP NOT NULL;