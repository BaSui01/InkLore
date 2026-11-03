# InkLore Staging: Railway + isolated Supabase

This is a **deployment blueprint**, not a production release or proof of a
running Staging URL. Do not enable public domains before secrets, TLS,
database privileges and Creator/Admin origins have been verified.

## Environment boundary

- Deploy only the reviewed `dev` commit, into a separate Railway
  **inklore-staging** project/environment. Never target the stable
  `private` branch, the production DB, or a shared Redis.
- PostgreSQL: existing Supabase project `pgsql` (Tokyo) has a private
  `inklore_ci` schema with 17 source-verified migrations and 80 tables.
  It contains no business data as of the inspection. Do **not** rerun R1
  migrations or seed users automatically.
- The Supabase `anon`, `authenticated` and `service_role` roles have
  no USAGE on `inklore_ci`. The application must use a separate,
  restricted **LOGIN** identity with only the required
  `inklore_runtime` privileges; never use Supabase's `postgres`
  administrator login for a long-running web service.
- Use a dedicated managed Railway Redis service in the same Staging
  environment. Never share any production cache, queue or API key.
- No live LLM API credentials are required for initial health, login
  and fake-model acceptance checks. Feature paths needing a provider
  must fail closed until a scoped staging-only key is configured.

## Service topology

Create one Railway project with four services in the **same environment**.
All three application services use source `BaSui01/InkLore`, branch
`dev`, repository root directory `/`, and the Dockerfile paths below.

| Service name | Dockerfile path (repository root relative) | Public domain |
| --- | --- | --- |
| `inklore-api` | `deploy/staging/Dockerfile.api` | **No** (private only) |
| `inklore-creator` | `deploy/staging/Dockerfile.creator` | Yes, after secrets and origin check |
| `inklore-admin` | `deploy/staging/Dockerfile.admin` | Restricted to reviewers / staging testers |
| `redis` | Railway managed Redis | **No** |

Set `RAILWAY_DOCKERFILE_PATH` to the matching path for each
GitHub-source service. Do not set a frontend-only root directory because
both SPA images need the shared `frontend/packages` workspace. The
Dockerfile-specific ignore files include frontend sources, unlike the
root `.dockerignore` used for the Go image.

### `inklore-api` environment variables

```dotenv
APP_ENV=production
HTTP_ADDR=0.0.0.0:8080
APP_HOST=0.0.0.0
APP_PORT=8080
PORT=8080
MIGRATE_ON_START=false
DATABASE_DRIVER=postgres
# DATABASE_DSN must be a SECRET, not committed. Configure TLS and the
# URL-encoded search_path=inklore_ci,extensions,public for the scoped role.
REDIS_ENABLED=true
REDIS_ADDR=redis.railway.internal:6379
REDIS_HOST=redis.railway.internal
REDIS_PORT=6379
COOKIE_SECURE=true
# Both frontend proxies forward the original client address in
# X-Forwarded-For. Trusting the private network lets c.ClientIP()
# recover it, keeping the per-IP login limiter keyed on real clients.
# 10.0.0.0/8 covers the Railway private network; narrow it to the
# actual assigned range if your workspace provides one.
API_TRUSTED_PROXY_CIDRS=10.0.0.0/8
```

Configure these additional **independent, random, at least 32-byte**
keys as encrypted Railway variables: `JWT_SIGNING_KEY`, `CSRF_KEY`,
`IDEMPOTENCY_HASH_KEY`, `CURSOR_SIGNING_KEY`. Provide
`ENCRYPTION_KEY` only through the secret store if the enabled adapter
requires it. Set `REDIS_PASSWORD` through a Railway reference
variable if the Redis service uses authentication.

After the domains are created, set both exact HTTPS origins (no trailing
slash) on the API service: `CREATOR_ORIGIN` and `ADMIN_ORIGIN`.
They must be different. The server validates these values on boot.
Do not use wildcard CORS. The source template copied into the
staging-only API image is not a credential file.

The Supabase connection URI must use a restricted database user,
TLS (`sslmode=require` or `verify-full`) and a URL-encoded
PostgreSQL search-path option, for example:
`options=-csearch_path%3Dinklore_ci%2Cextensions%2Cpublic`.
Verify the URI with a read-only query against the exact target,
never paste it into a PR, chat transcript or application log.

### `inklore-creator` and `inklore-admin` environment

```dotenv
PORT=8080
API_UPSTREAM=http://inklore-api.railway.internal:8080
```

The Nginx proxy preserves `/v1/*` on the *same web origin*, including
SSE and WebSocket upgrades. The API therefore does not require its own
public Railway domain. Creator and Admin are separate static bundles
and must have separate public hostnames. The web containers refuse to
start if `API_UPSTREAM` is missing. `API_UPSTREAM` is re-resolved at
runtime through the container DNS resolver (extracted from
`/etc/resolv.conf` at start), so API redeploys that change the private
address are picked up without restarting the frontend containers.

## Safe deployment order and acceptance

1. Verify Railway project, environment, plan and consumption limits
   before provisioning resources that might incur charges.
2. Inspect the existing Supabase `inklore_ci` schema and grant a
   dedicated least-privilege LOGIN role. Obtain its URI via provider
   secret storage, not GitHub. Confirm no access to `public` business
   tables and no Data API exposure.
3. Start private Redis and the API with its secrets, exact origins and
   `MIGRATE_ON_START=false`. Require `/healthz` 200 and `/readyz`
   reporting database/Redis ready.
4. Start Creator and Admin web services. Only then assign HTTPS
   testing domains; verify each root and login page returns its
   respective SPA HTML, `/assets/missing.js` is 404, and `/v1/auth/me`
   returns JSON 401 without credentials.
5. Use separate synthetic users/tenants in Staging. Exercise login,
   novel editing and save/reload, Agent Run/Job persistence, tenant
   denial, SSE replay and explicit failure cases. Do not create real
   paid-model calls until separately authorized. Capture redacted
   browser and HTTP evidence.
6. Enable GitHub deployment gates only after these checks pass.
   Keep environments and credentials independent from production.

**Rollback:** disable frontend public domains first; roll app images
back to the previous reviewed commit, keeping migrations frozen.
Do not reset the database or run down migrations on a shared Supabase
project.

## Remaining blockers

This manifest does not provision paid Railway resources, generate
security secrets, create database users, or assert a Staging URL.
Those operations require the connected Railway deployment action,
validated account permissions and resource-cost review. A green
GitHub Actions run validates images, not deployed browser behavior.
