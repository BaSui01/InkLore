#!/usr/bin/env python3
"""Creator/Admin/Agent API acceptance against a disposable PostgreSQL/Redis stack.

Runs only on GitHub-hosted ephemeral CI infrastructure with generated identities.
No real provider keys, production data, or shared Supabase writes are used.
"""
import json
import os
import secrets
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

BASE = os.getenv("E2E_API_URL", "http://127.0.0.1:18080").rstrip("/")
CREATOR_ORIGIN = "http://127.0.0.1:3000"
ADMIN_ORIGIN = "http://127.0.0.1:3001"

def check(condition, message):
    if not condition:
        raise AssertionError(message)

def call(path, method="GET", body=None, token=None, origin=None, key=None):
    headers = {"Accept": "application/json"}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body, separators=(",", ":")).encode()
    if token:
        headers["Authorization"] = "Bearer " + token
    if origin:
        headers["Origin"] = origin
    if key:
        headers["Idempotency-Key"] = key
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=12) as res:
            raw = res.read(2 * 1024 * 1024)
            return res.status, json.loads(raw)
    except urllib.error.HTTPError as err:
        raw = err.read(2 * 1024 * 1024)
        try:
            return err.code, json.loads(raw)
        except (ValueError, UnicodeError):
            return err.code, {}

def require(path, status, *, method="GET", body=None, token=None, origin=None, key=None):
    got, result = call(path, method, body, token, origin, key)
    check(got == status, f"{method} {path}: expected HTTP {status}, got {got}, code={result.get('code', 'unknown')}")
    if status < 300:
        check(result.get("code") == "ok", f"{path}: invalid success envelope")
        return result["data"]
    return result

def main():
    admin_email = os.environ["E2E_ADMIN_EMAIL"]
    admin_password = os.environ["E2E_ADMIN_PASSWORD"]
    creator_email = os.environ["E2E_CREATOR_EMAIL"]
    creator_password = os.environ["E2E_CREATOR_PASSWORD"]
    run_suffix = secrets.token_hex(8)

    require("/v1/auth/me", 401)
    require("/v1/content/novels", 401)
    require("/v1/agents/definitions", 401)
    admin_base = require(
        "/v1/auth/login", 200, method="POST", origin=ADMIN_ORIGIN,
        body={"email": admin_email, "password": admin_password, "audience": "admin"},
    )
    check(admin_base["principal"]["audience"] == "admin", "Admin bootstrap login audience mismatch")
    print("Admin: independent deployment bootstrap and password login passed.")

    creator = require(
        "/v1/auth/register", 200, method="POST", origin=CREATOR_ORIGIN,
        body={"email": creator_email, "password": creator_password, "pen_name": "CiCreator"},
    )
    other = require(
        "/v1/auth/register", 200, method="POST", origin=CREATOR_ORIGIN,
        body={"email": f"ci-other-{run_suffix}@example.invalid",
              "password": secrets.token_urlsafe(24), "pen_name": "CiOther"},
    )
    require(
        "/v1/auth/register", 409, method="POST", origin=CREATOR_ORIGIN,
        body={"email": creator_email, "password": creator_password, "pen_name": "CiCreator"},
    )
    creator_token = creator["access_token"]
    other_token = other["access_token"]
    tenant = creator["principal"]["tenant_id"]
    check(tenant != other["principal"]["tenant_id"], "creator registrations did not isolate tenants")
    me = require("/v1/auth/me", 200, token=creator_token)
    check(me["tenant_id"] == tenant, "Creator session tenant mismatch")

    novel = require(
        "/v1/content/novels", 201, method="POST", token=creator_token,
        key=str(uuid.uuid4()), origin=CREATOR_ORIGIN,
        body={"title": "CI Isolation Novel", "summary": "Ephemeral acceptance",
              "category": "fantasy", "target_words": 40000},
    )
    novel_id = novel["id"]
    check(require(f"/v1/content/novels/{novel_id}", 200, token=creator_token)["id"] == novel_id,
          "Creator novel round-trip failed")
    require(f"/v1/content/novels/{novel_id}", 404, token=other_token)
    check(novel_id not in json.dumps(require("/v1/content/novels", 200, token=other_token)),
          "Tenant B can list Tenant A's novel")
    print("Creator: registration, session, persisted novel and cross-tenant denial passed.")

    admin = require(
        "/v1/auth/login", 200, method="POST", origin=ADMIN_ORIGIN,
        body={"email": admin_email, "password": admin_password, "audience": "admin",
              "tenant_id": tenant},
    )
    admin_token = admin["access_token"]
    check(admin["principal"]["tenant_id"] == tenant, "Root Admin tenant projection unavailable")
    require("/v1/agents/definitions", 200, token=admin_token)
    require("/v1/agents/builtin-tools", 200, token=admin_token)
    bad_admin, _ = call("/v1/agents/definitions", token=creator_token)
    check(bad_admin in (401, 403, 404), "Creator token accessed Admin Agent governance")
    print("Admin: cross-tenant root membership, governance catalog and audience gate passed.")

    title = "CI Novel Architect " + run_suffix
    definition = require(
        "/v1/agents/definitions", 201, method="POST", token=admin_token,
        origin=ADMIN_ORIGIN, key=str(uuid.uuid4()),
        body={"definition_key": "ci_architect_" + run_suffix,
              "title": title, "enabled": True,
              "system_instruction": "Help with outline safely in CI.",
              "tool_version_ids": [], "task_kinds": ["outline_suggest"]},
    )
    definition_id = definition["id"]
    check(require(f"/v1/agents/definitions/{definition_id}", 200,
                  token=admin_token)["id"] == definition_id,
          "Created Agent definition could not be read")
    own_catalog = require("/v1/agents/catalog/definitions", 200, token=creator_token)
    check(any(x["id"] == definition_id for x in own_catalog["items"]),
          "Published Agent not available to owning creator")
    other_catalog = require("/v1/agents/catalog/definitions", 200, token=other_token)
    check(not any(x["id"] == definition_id for x in other_catalog["items"]),
          "Published Agent leaked to another tenant")
    require("/v1/agents/runs", 200, token=creator_token)

    # Real source-ownership check must run BEFORE any workflow or model dispatch.
    bad_resource = {
        "task_kind": "outline_suggest", "task_key": "outline_suggest",
        "definition_id": definition_id, "instruction": "Test source access",
        "resource": {
            "resource_id": str(uuid.uuid4()), "resource_type": "novel",
            "novel_id": novel_id, "source_revision": 1,
        },
    }
    code, _ = call("/v1/agents/runs", "POST", bad_resource, creator_token,
                   CREATOR_ORIGIN, str(uuid.uuid4()))
    check(code in (400, 403, 404, 409, 422),
          f"Unauthorized Agent resource did not fail closed: HTTP {code}")
    # The other tenant must not even read a valid creator-owned novel.
    other_run = dict(bad_resource)
    other_run["resource"] = {"resource_id": novel_id, "resource_type": "novel",
                             "novel_id": novel_id, "source_revision": 1}
    code, _ = call("/v1/agents/runs", "POST", other_run, other_token,
                   CREATOR_ORIGIN, str(uuid.uuid4()))
    check(code in (403, 404), f"Other tenant Agent run crossed isolation: HTTP {code}")
    run_list = require("/v1/agents/runs", 200, token=creator_token)
    check(not run_list["items"], "Rejected Agent runs created persisted jobs")
    print("Agent: Admin definition write/read, Creator catalog, tenant isolation and denied Run rollback passed.")

    fixture_path = Path(os.getenv("E2E_FIXTURE_PATH", "/tmp/inklore-e2e-credentials.json"))
    fd = os.open(str(fixture_path), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf8") as file:
        json.dump({"creator_email": creator_email, "creator_password": creator_password,
                   "admin_email": admin_email, "admin_password": admin_password,
                   "tenant_id": tenant, "definition_title": title}, file)
    print("Creator/Admin/Agent HTTP + PostgreSQL end-to-end acceptance passed.")

if __name__ == "__main__":
    try:
        main()
    except (AssertionError, KeyError, TimeoutError, urllib.error.URLError) as exc:
        # Do not dump raw bodies, tokens, passwords or DB connection strings.
        print("Acceptance failed: " + str(exc).replace(BASE, "[API]")[:320], file=sys.stderr)
        sys.exit(1)
