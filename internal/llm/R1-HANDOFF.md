# INK-167 bounded implementation handoff

Status: partial implementation, not accepted and not done. The R1 HTTP management transport is now present and app assembly is parent-owned. No Budget ledger, Redis Gateway, or real provider driver is connected. Legacy APIs remain unchanged and must not be used as R1 fallbacks.

## Stable constructors

- infrastructure/r1.NewStore(manager *uow.Manager) (*Store,error): domain/r1.CatalogRepository implementation. Uses only llm-owned tables. All mutations require an active platform UoW. Application acquires a tenant catalog advisory lock before writes.
- application/r1.NewInputValidator(common,llm []byte) (func(domain/r1.Kind,[]byte)error,error): caller supplies frozen common.json and llm-schemas.json bytes; pinned registry uses no external I/O. Strict JSON decoding precedes full input schema validation.
- infrastructure/r1.NewSecrets(SecretOptions) (*Secrets,error): deployment supplies Bindings []SecretBinding{ProviderID,Ref}, Endpoints map[protocol][]exactURL, Pepper of at least 32 bytes, and Resolve(context.Context,SecretBinding)([]byte,error). Resolver returns an owned slice cleared after use and must itself avoid logging secret values. Fingerprint is provider-bound HMAC-SHA256; arbitrary references are denied before resolution.
- application/r1.NewCatalog(CatalogOptions) (*Catalog,error): requires Repository, UoW, Identity, Secrets, ValidateInput and OnMutation. Identity implements Authorize(context.Context,identity.Principal,string,...string)(identity.Principal,error). UoW implements Within(context.Context,func(context.Context)error)error. OnMutation(context.Context,Mutation)error must append audit/outbox in the same transaction.

Catalog methods: Authorize(ctx,p,write), Get(ctx,p,kind,id), List(ctx,p,kind,ListQuery), Write(ctx,p,kind,id,expected,raw), Delete(ctx,p,kind,id,expected). Write with empty id creates; nonempty id requires expected_revision. Policy PUT inserts a new version and atomically switches the head. Pricing is insert-only and rejects past effective_at. Provider protocol is immutable. Referenced model capability changes are conservatively rejected. Secret metadata contains no plaintext.

List takes already authenticated cursor anchors. Future transport must validate filters, limit, date window and cursor tenant/actor/permission binding; it must wrap all mutations with platform idempotency and reauthorize replay. The raw input passed to Write contains the schema Input fields only: transport must separately parse/remove expected_revision. No HTTP Handler/Options were implemented in this slice.

Strict output API: CompileOutput(schema []byte) (*StrictOutput,error), Validate(raw []byte)error and DecodeStrictJSON(raw,limit,depth). Rejects invalid UTF8, duplicate JSON properties, trailing text, outside refs and unknown schema keywords; root object/array, schema <=32KiB/depth16, output <=2MiB, RE2 regex, bounded schema traversal, recursive references rejected. Supports local $defs references. Unknown format behavior and resource-budget performance still require runtime acceptance. Errors never include output payload. No upstream repair, Markdown stripping or automatic retry is introduced.

Accounting API: domain/r1.Cost(PriceSnapshot,Tokens) (int64,error) performs exact big.Int intermediate arithmetic and independent ceil of uncached input/cache/output costs. Values and reservation/settlement types are present, but no persisted budget transition has been implemented.

## Exact verification

After repairing an initial accidental NUL in secrets.go, the read tool successfully read that file as UTF-8 (106 lines). Latest command: gofmt -w internal/llm/domain/r1 internal/llm/application/r1 internal/llm/infrastructure/r1; go build ./internal/llm/... . Exit code 0. Scoped git diff --check exit code 0. All added Go files are below 50KB. No tests, full repository builds, PG commands, network calls or real supplier calls were run.

## Required remaining work

1. App composition and runtime acceptance of the completed HTTP transport; 32 routes are now mounted, with 30 Catalog/pricing/usage operations and two explicit 503 Budget stubs.
2. Budget Provision/Get/Configure CAS and Beijing day buckets; PostgreSQL Reserve/Dispatch/Settle/Release/Adjustment/Recovery; immutable usage receipts and price snapshots.
3. Gateway authorization/policy resolution, Redis atomic channel/key SWRR/RPM/leases/circuit state, failclosed recovery quarantine and dispatch cancellation. No Redis failclosed proof currently exists.
4. Real provider adapters with TLS/redirect/host binding, normalized terminal semantics, no uncertain-outcome retries, and actual capability support checks. Catalog currently checks declared model capabilities, not driver implementation capability.
5. PG verification of tenant/CAS/rollback/version/head/price correctness, strict-schema positive/negative runtime acceptance, SecretRef security checks, race/concurrency/budget/time-boundary evidence.
6. Parent-owned migration/app/governance document integration. This subtask did not change app, platform, docs, DDL or dependencies.
