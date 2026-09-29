-- 000009：Provider / 模型 / 渠道注册表种子数据（INK-019）
--
-- 背景：注册表从内存实现切换到 PostgreSQL（INK-019）后，管理端不再有进程内种子数据。
-- 本迁移把原先内存实现内置的 4 个 Provider / 5 个模型 / 4 个渠道写入数据库，保持管理端
-- 初始视图与切换前一致。全部语句幂等（ON CONFLICT DO NOTHING / NOT EXISTS），可重复执行。
--
-- 注意：这里只写元数据，不含任何密钥字段 —— API Key 只从环境变量注入（ADR-0004）。

INSERT INTO llm_providers (code, display_name, protocol, enabled) VALUES
    ('openai',    'OpenAI',           'openai',    TRUE),
    ('deepseek',  'DeepSeek',         'openai',    TRUE),
    ('anthropic', 'Anthropic Claude', 'anthropic', TRUE),
    ('gemini',    'Google Gemini',    'gemini',    TRUE)
ON CONFLICT (code) DO NOTHING;

INSERT INTO llm_models (provider_id, model_key, capabilities, enabled)
SELECT p.id, v.model_key, v.capabilities::jsonb, TRUE
FROM (VALUES
    ('openai',    'gpt-4o',              '["text", "stream", "tool_calling", "structured_output"]'),
    ('deepseek',  'deepseek-chat',       '["text", "stream"]'),
    ('deepseek',  'deepseek-reasoner',   '["text", "stream", "reasoning"]'),
    ('anthropic', 'claude-3-7-sonnet',   '["text", "stream", "tool_calling"]'),
    ('gemini',    'gemini-2.0-flash',    '["text", "stream", "multimodal"]')
) AS v(code, model_key, capabilities)
JOIN llm_providers p ON p.code = v.code
ON CONFLICT (provider_id, model_key) DO NOTHING;

INSERT INTO llm_channels (provider_id, name, base_url, routing_weight, enabled)
SELECT p.id, v.name, v.base_url, 100, TRUE
FROM (VALUES
    ('openai',    'OpenAI Official Gateway', 'https://api.openai.com/v1'),
    ('deepseek',  'DeepSeek Official API',   'https://api.deepseek.com'),
    ('anthropic', 'Anthropic Official API',  'https://api.anthropic.com'),
    ('gemini',    'Google AI Studio',        'https://generativelanguage.googleapis.com/v1beta')
) AS v(code, name, base_url)
JOIN llm_providers p ON p.code = v.code
WHERE NOT EXISTS (
    SELECT 1 FROM llm_channels c WHERE c.provider_id = p.id AND c.name = v.name
);