#!/bin/sh
set -eu

# Never fall back to the example configuration's placeholder database URL.
: "${DATABASE_DSN:?Set a restricted Supabase staging DSN in Render secrets}"
: "${REDIS_URL:?Render internal Key Value URL is required}"

case "$DATABASE_DSN" in
  *inklore_ci*) ;;
  *) echo 'Refusing database URL without private inklore_ci search_path' >&2; exit 1 ;;
esac
case "$DATABASE_DSN" in
  *sslmode=require*|*sslmode=verify-full*) ;;
  *) echo 'Refusing database URL without required TLS' >&2; exit 1 ;;
esac

# Render Free Key Value's internal URL is unauthenticated by default. Do not
# strip credentials or silently fall back to an external Redis endpoint.
case "$REDIS_URL" in
  redis://*) redis_hostport="${REDIS_URL#redis://}" ;;
  *) echo 'Expected internal redis:// Key Value URL' >&2; exit 1 ;;
esac
case "$redis_hostport" in
  ''|*[@/?#]*) echo 'Unsupported Redis internal URL: refusing fallback' >&2; exit 1 ;;
esac
export REDIS_ADDR="$redis_hostport"
exec /app/server
