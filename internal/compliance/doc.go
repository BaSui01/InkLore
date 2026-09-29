// Package compliance owns tenant rule sets, inspection reports, issues,
// decision records, risk review receipts, and sanitization rewrite suggestions.
//
// In R1, compliance operates as an author-facing self-check toolkit rather than
// an external censorship authority. It provides deterministic fast scanning
// (via local Unicode rune matching and RE2 regular expressions without LLM invocation),
// deep inspection, non-invasive rewrite suggestions (which never write back directly
// to content chapters or novel bodies), and immutable review receipts required for export.
//
// The domain is structured into four layers under r1:
//   - domain/r1: Entities, values, matching automaton, scoring, patches, and repository interfaces.
//   - application/r1: Use case orchestration, ports, execution lifecycle, and review validation.
//   - infrastructure/r1: PostgreSQL persistence with tenant isolation, UoW, and snapshot caching.
//   - transport/http/r1: REST HTTP routes, guards, authentication, and idempotency handling.
package compliance
