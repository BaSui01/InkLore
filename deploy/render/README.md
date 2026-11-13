# InkLore: Render Free Staging (INK-255)

Scope: **free test hosting**, not production, not 24-hour background execution.
The four-resource blueprint is in [render.yaml](../../render.yaml).

## Layout and exact free plans

| Render resource | Runtime | Plan | Purpose |
| --- | --- | --- | --- |
| inklore-basui01-staging-api | Docker/Go | `free` | API and its existing embedded Agent Worker |
| inklore-basui01-staging-creator | Static | Free static site | Creator Vite React bundle |
| inklore-basui01-staging-admin | Static | Free static site | Admin Vite React bundle |
| inklore-basui01-staging-kv | Key Value | `free`, 25 MB | Loss-tolerant Redis compatibility |

Both frontend sites are static, not paid Web instances. PostgreSQL remains
in the existing Supabase `pgsql` project, private `inklore_ci` schema,
which has 17 verified migrations and 80 business tables. No migrations, data
destruction, fixture loading, or account seeding are executed on deployment.

Only Render's Free plans are declared. A Free Web instance sleeps after 15
minutes idle and takes time to wake; 750 free instance-hours are shared by
free Web services in a workspace. Free Key Value does not persist across
restarts: never use it for durable queues or a 24/7 Worker claim. The default
maxmemory policy is explicitly `noeviction`. Free bandwidth/build quotas
still apply, and the platform may suspend free services when exceeded.

## Prepare trusted database access

* Confirm the Supabase project and private `inklore_ci` schema are the
  intended **test** database, not an existing production dataset.
* Create a separate, minimal LOGIN role for the application, scoped to the
  `inklore_ci` schema and required tables. Do not use the Supabase
  `postgres` administrative connection for a public web process.
* Give Render the restricted connection URI using its secure initial
  Blueprint variable prompt, with required TLS and the exact PostgreSQL
  search path `inklore_ci,extensions,public`. Do **not** put the URI or its
  password in GitHub, Render routes, screenshots, browser console or chat.
  When using a URI, the search-path connection option is URL encoded
  (`options=-csearch_path%3Dinklore_ci%2Cextensions%2Cpublic`).
* `deploy/render/entrypoint.sh` rejects missing/non-TLS/non-staging DSNs
  and malformed Key Value URLs before launching the server.
* The four signing keys in `render.yaml` use four independent generated
  values. No real model API key is configured; model calls must fail closed.

## Deployment sequence

1. Merge the reviewed blueprint PR into `dev` after CI. Use Render
   Dashboard > **New > Blueprint**, connect `BaSui01/InkLore`, select
   branch `dev`, inspect all four resources and ensure every selected
   compute plan is Free. Do not select a paid plan, extra database, disk
   or background worker.
2. At the Blueprint's **initial** creation, provide
   `DATABASE_DSN` in Render's secret prompt. All other signing secrets
   are generated automatically. The Render Key Value internal connection
   URL is provided through a `fromService` reference. The Go entrypoint
   converts the unauthenticated internal `redis://host:port` URL into
   `REDIS_ADDR`; it refuses to silently discard authentication if Render
   changes its Key Value configuration.
3. The Blueprint's `autoDeployTrigger: off` avoids unintended build
   consumption from new commits. Manually deploy the reviewed commit.
4. Before browser testing, check the **actual** Render-assigned service
   hostnames. The Blueprint uses candidate `onrender.com` names; they
   are not guaranteed until Render allocates them. If different, update
   both frontend `/v1/*` rewrites and the API's exact
   `CREATOR_ORIGIN`/`ADMIN_ORIGIN` values. Resync and redeploy.
5. Verify the API `/healthz` and `/readyz` return 200. A public API
   domain is required because Render Free Web instances cannot receive
   private service-to-service traffic from the static-site CDN.
   Authentication and origin enforcement remain application-owned.
6. Verify each site loads its own login SPA, that unauthenticated
   `/v1/auth/me` returns JSON 401 (not an HTML fallback), and that
   an authorized creator can register, log in, refresh and log out.
   Check the `Set-Cookie`/Origin/CSRF behavior through the **actual**
   external static-site rewrite and POST method. Rewrites are not proof
   of browser authentication until this passes.
7. Verify editor save/reload, cross-tenant denial, Agent Run/Job
   persistence and SSE reconnect. Render's static CDN is not a guaranteed
   WebSocket gateway; test WebSocket upgrades separately against the
   real public Web Service endpoint and the protected origin contract.
   Retain GitHub Actions' isolated PG18/Redis7 suite for deterministic
   fault, concurrency and Worker recovery behavior.
8. Inspect Render's monthly included usage after each iteration.
   Never enable paid upgrades automatically.

## Operations and limitations

**No 24h unattended worker guarantee.** On Render Free, idle API sleeps and
Redis state can disappear. Persistent business state belongs to Supabase,
but scheduled work cannot assume it will continue during API sleep.

**No production deployment.** Never connect the stable `private`
branch or real payment/model credentials to this blueprint.

**Rollback.** Disable application deploys, redeploy the previous reviewed
commit; do not execute a destructive database migration or clear Supabase
data. Free Key Value is ephemeral and may have lost state.

**Not yet deployed:** a Blueprint in Git is an implementation artifact,
not evidence of a running Render service, allocated URL or browser E2E.

Official references:
* https://render.com/docs/blueprint-spec
* https://render.com/docs/free
* https://render.com/docs/redirects-rewrites
* https://render.com/docs/key-value
