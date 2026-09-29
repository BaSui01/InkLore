// Package inspiration owns reference assets, segments, analysis requests,
// reports, versioned style profiles, inspiration cards, and adoption references.
//
// All inspiration artifacts are external suggestions. They never directly write
// to content, outline, or worldview domain facts or bodies. Adoptions record
// immutable references to validated target revisions and can be revoked, but
// revocation never rolls back or mutates the target novel text or facts.
//
// Subpackages:
//   - domain/r1: Value types, invariants, chunking, and normalization.
//   - application/r1: Use cases, ports, and lifecycle orchestration.
//   - infrastructure/r1: Persistence scoped strictly to inspiration tables.
//   - transport/http/r1: HTTP REST handlers and route registrations.
package inspiration
