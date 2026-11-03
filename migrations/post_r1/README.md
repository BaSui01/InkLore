# Post-R1 platform extensions

This directory is NOT included in `migrations/r1.FS()` or the existing `cmd/migrate` R1 runner.
The 17 frozen R1 migration sources and their checksums must remain untouched.

## INK-273 first stage

`000001_platform_mail_settings.sql` is a proposed *next-stream* migration
for two DB-owned objects:

- `platform_mail_settings`: global Admin mail-provider configuration, CAS
  `revision`, credentials encrypted with an external key or referenced
  through an environment variable. No plaintext stored or exposed by APIs.
- `platform_email_challenges`: hashed OTP with explicit
  audience/purpose/tenant scope, expiration, attempt count and one-time consumption.
  It must remain durable even if Render Free Redis restarts.

The migration uses the current SQL search path. When applied to a dedicated
Supabase staging schema, the execution identity MUST set it to
`inklore_ci,extensions,public` before running the file and verify the exact
schema and database identity. When applied to an ephemeral CI database, it
runs in `public`. The migration **does not belong to the R1 stream**; the
new post-R1 migration metadata/runner and release gate will be integrated
only with the reviewed mail runtime, not silently invoked on server startup.

The initial scope is **schema + constraints + privileges only**. No SMTP
sending, Resend HTTP API, Admin page, email OTP authentication or password
recovery is enabled by this SQL alone. Those must share current Identity,
AuthSession, httpguard/requestschema, audit and GORM/UoW, and undergo
separate browser and provider mock acceptance before production rollout.

For isolated verification, use the dedicated GitHub Actions
`.github/workflows/mail-persistence.yml`: it provisions disposable PostgreSQL
17, runs the 17 original migrations, applies the extension once and checks
schema constraints/permissions and atomic one-time OTP consumption.

Do not run this SQL directly against the shared Supabase project before
the full mail feature contract and database migration gate have been reviewed.
