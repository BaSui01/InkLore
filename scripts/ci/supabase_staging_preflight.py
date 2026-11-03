#!/usr/bin/env python3
"""Read-only Supabase staging preflight for an IPv4-only GitHub-hosted runner.

Credentials stay in process environment. Never log a DSN, password, or psql stderr.
The shared pooler host cannot be inferred from a region: validate each candidate
by successfully authenticating into this exact project and querying the role.
"""

import os
import subprocess
import sys
from urllib.parse import parse_qs, unquote, urlsplit

REF = "rjxtggttklpqbfjesrgp"
ROLE = "inklore_ci_preflight"
DIRECT_HOST = f"db.{REF}.supabase.co"
SCHEMA = "inklore_ci"
REGION = "ap-northeast-1"

CHECK = """
SELECT CASE WHEN
  current_user = 'inklore_ci_preflight'
  AND current_database() = 'postgres'
  AND (SELECT count(*) FROM inklore_ci.schema_migrations) = 17
  AND (SELECT count(*) FROM pg_tables WHERE schemaname = 'inklore_ci') = 80
  AND NOT has_schema_privilege('anon', 'inklore_ci', 'USAGE')
  AND NOT has_schema_privilege('authenticated', 'inklore_ci', 'USAGE')
  AND NOT has_schema_privilege('service_role', 'inklore_ci', 'USAGE')
  AND EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')
  AND EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')
  AND has_table_privilege(current_user, 'inklore_ci.schema_migrations', 'SELECT')
  AND NOT has_table_privilege(current_user, 'inklore_ci.schema_migrations', 'UPDATE')
  AND NOT has_table_privilege(current_user, 'inklore_ci.schema_migrations', 'INSERT')
  AND NOT has_table_privilege(current_user, 'inklore_ci.schema_migrations', 'DELETE')
THEN 'PASS' ELSE 'FAIL' END;
"""

def main() -> int:
    dsn = os.environ.get("STAGING_DSN", "")
    if not dsn:
        print("Missing staging connection secret.", file=sys.stderr)
        return 2
    try:
        uri = urlsplit(dsn)
        user = unquote(uri.username or "")
        password = unquote(uri.password or "")
        options = parse_qs(uri.query)
        sslmode = options.get("sslmode", [""])[0]
        if (uri.scheme not in ("postgres", "postgresql")
            or uri.hostname != DIRECT_HOST
            or uri.port != 5432
            or uri.path != "/postgres"
            or user != ROLE
            or not password
            or sslmode not in ("require", "verify-full")):
            raise ValueError("Unexpected connection parameters")
    except (ValueError, TypeError):
        print("Staging URL does not match the expected private role and TLS settings.", file=sys.stderr)
        return 2

    # Every host is under the official Supabase pooler domain. Only a real
    # authenticated query can establish which one serves this project.
    for index in range(4):
        host = f"aws-{index}-{REGION}.pooler.supabase.com"
        env = dict(os.environ)
        env.pop("STAGING_DSN", None)
        env.update({
            "PGHOST": host,
            "PGPORT": "5432",
            "PGDATABASE": "postgres",
            "PGUSER": f"{ROLE}.{REF}",
            "PGPASSWORD": password,
            "PGSSLMODE": sslmode,
            "PGCONNECT_TIMEOUT": "6",
        })
        try:
            p = subprocess.run(
                ["psql", "-X", "-w", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", CHECK],
                env=env, capture_output=True, text=True, timeout=15, check=False,
            )
        except (OSError, subprocess.TimeoutExpired):
            continue
        if p.returncode == 0 and p.stdout.strip() == "PASS":
            print(f"Supabase staging preflight passed using verified IPv4 session pooler: {host}")
            return 0

    print(
        "No verified IPv4 session pooler passed. In Supabase > Connect > Session pooler "
        "confirm the host; credentials and the direct IPv6 URL were not printed.",
        file=sys.stderr,
    )
    return 1

if __name__ == "__main__":
    sys.exit(main())
