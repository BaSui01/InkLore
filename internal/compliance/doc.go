// Package compliance owns tenant rule sets, inspection reports, issues,
// decision records, risk review receipts, and sanitization rewrite suggestions.
//
// compliance operates as an author-facing self-check toolkit rather than
// an external censorship authority. It provides deterministic fast scanning
// (via local Unicode rune matching and RE2 regular expressions without LLM invocation),
// deep inspection, non-invasive rewrite suggestions (which never write back directly
// to content chapters or novel bodies), and immutable review receipts required for export.
//
// The domain is structured into four layers:
//   - domain: Entities, values, matching automaton, scoring, patches, and repository interfaces.
//   - application: Use case orchestration, ports, execution lifecycle, and review validation.
//   - infrastructure: PostgreSQL persistence with tenant isolation, UoW, and snapshot caching.
//   - transport/http: REST HTTP routes, guards, authentication, and idempotency handling.
package compliance
