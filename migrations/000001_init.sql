-- Extensions for Vector Search (RAG) and Fuzzy Text Matching
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- LLM Provider & Model Registry
CREATE TABLE llm_providers (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    protocol TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE llm_models (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES llm_providers(id),
    model_key TEXT NOT NULL,
    capabilities JSONB NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(provider_id, model_key)
);

CREATE TABLE llm_channels (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES llm_providers(id),
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    routing_weight INTEGER NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE llm_usage_records (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    model_key TEXT NOT NULL,
    channel_id BIGINT REFERENCES llm_channels(id),
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Agent Definitions & Event-Sourced Runs (DSH Traceable Sessions)
CREATE TABLE agent_definitions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    default_model TEXT NOT NULL,
    tools JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_runs (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT REFERENCES agent_definitions(id),
    request_id TEXT NOT NULL,
    status TEXT NOT NULL,
    input_snapshot JSONB NOT NULL,
    output_snapshot JSONB,
    trace_events JSONB NOT NULL DEFAULT '[]',
    full_context_snapshot JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_runs_trace ON agent_runs USING gin (trace_events);
CREATE INDEX idx_agent_runs_request_id ON agent_runs (request_id);

-- Novel Domain Entities
CREATE TABLE novels (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    synopsis TEXT NOT NULL DEFAULT '',
    world_bible TEXT NOT NULL DEFAULT '',
    genre TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE novel_characters (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    profile JSONB NOT NULL DEFAULT '{}',
    embedding vector(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_novel_characters_name ON novel_characters USING gin (name gin_trgm_ops);

CREATE TABLE novel_chapters (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    title TEXT NOT NULL,
    outline TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    embedding vector(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(novel_id, ordinal)
);

-- Fact Ledger (Canonical Facts, World Rules, Clues & Timeline)
CREATE TABLE fact_ledger (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    category TEXT NOT NULL, -- 'rule', 'character_state', 'clue', 'timeline'
    fact_key TEXT NOT NULL,
    fact_value JSONB NOT NULL DEFAULT '{}',
    source_chapter_id UUID REFERENCES novel_chapters(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fact_ledger_novel ON fact_ledger (novel_id, category);
