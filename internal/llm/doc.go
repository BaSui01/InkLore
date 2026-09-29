// Package llm owns AI provider, model, channel, and key metadata,
// task policy versions, token pricing revisions, daily tenant budgets, and immutable usage records.
//
// In R1, the LLM domain acts as the unified AI model gateway and dispatch center for InkLore:
//   - domain: Defines catalog entities (Provider, Model, Channel, Key, Policy, Pricing),
//     daily budget and accounting rules, model execution requests, streaming events, and tool call contracts.
//   - application: Orchestrates catalog CRUD, budget commits and reservations, strict input/output
//     JSON schema validation, and gateway dispatch to underlying provider routers.
//   - infrastructure: Implements PostgreSQL catalog and budget repositories (with advisory locking
//     and tenant isolation), Redis-backed atomic admission leases (via Lua scripts), and protocol
//     adapters for upstream providers (OpenAI, DeepSeek, Anthropic, Gemini, Ollama).
//   - transport/http: Exposes 32 admin REST endpoints under /v1/llm with httpguard strict validation.
//
// LLM has zero business-domain dependencies (R1 DAG per ADR-0016).
package llm
