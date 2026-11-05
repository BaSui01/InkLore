#!/usr/bin/env python3
"""Disposable #98 HTTP/PG checks and optional Creator/Admin Chromium journey.

Accounts are registered through HTTP; only their fixture roles are seeded in PG.
Mail stays disabled throughout. No success response or delivery is mocked, and
this journey does not claim OTP delivery or an external provider acceptance.
"""
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

BASE = os.getenv("E2E_API_URL", "http://127.0.0.1:18080").rstrip("/")
ORIGINS = {"creator": "http://127.0.0.1:3000", "admin": "http://127.0.0.1:3001"}
CHECKS = []
SETTINGS = "/v1/platform/mail-settings"
FIELDS = {"provider", "enabled", "smtp_host", "smtp_port", "smtp_security", "smtp_username", "sender_email", "sender_name"}
SUMMARY_FIELDS = FIELDS | {"has_credential", "credential_kind", "revision", "updated_at"}


def check(value, message):
    if not value:
        raise AssertionError(message)


def verified(message):
    CHECKS.append(message)
    print("PASS " + message, flush=True)


def request(path, expected=200, method="GET", body=None, session=None, key=None, cookies=None):
    headers = {"Accept": "application/json"}
    if session:
        headers.update(Authorization="Bearer " + session["access_token"],
                       Origin=ORIGINS[session["principal"]["audience"]])
        headers["X-CSRF-Token"] = session["csrf_token"]
    if key:
        headers["Idempotency-Key"] = key
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode()
    try:
        with urllib.request.urlopen(urllib.request.Request(BASE + path, method=method, data=data, headers=headers), timeout=20) as response:
            status, payload = response.status, json.load(response)
            if cookies is not None:
                for header in response.headers.get_all("Set-Cookie", []):
                    parsed = SimpleCookie()
                    parsed.load(header)
                    for name, cookie in parsed.items():
                        cookies.append({"name": name, "value": cookie.value,
                                        "domain": urllib.parse.urlparse(BASE).hostname,
                                        "path": cookie["path"] or "/",
                                        "httpOnly": bool(cookie["httponly"]),
                                        "secure": bool(cookie["secure"]),
                                        "sameSite": cookie["samesite"].capitalize() or "Lax"})
    except urllib.error.HTTPError as response:
        status, payload = response.code, json.load(response)
    check(status == expected, f"{method} {path} returned {status}, expected {expected}")
    if status >= 300:
        return payload
    check(payload.get("code") == "ok" and "data" in payload, "Invalid API success envelope")
    return payload["data"]


def sql(statement):
    # Bind psql to the already-validated DSN, never an unrelated PGHOST override.
    dsn = urllib.parse.urlparse(os.environ["DATABASE_DSN"])
    env = {**os.environ, "PGHOST": dsn.hostname, "PGPORT": str(dsn.port or 5432),
           "PGUSER": urllib.parse.unquote(dsn.username or ""), "PGDATABASE": dsn.path[1:],
           "PGPASSWORD": urllib.parse.unquote(dsn.password or "")}
    result = subprocess.run(["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1"], input=statement,
                            text=True, capture_output=True, env=env)
    check(result.returncode == 0, "Disposable mail UI fixture SQL failed; credentials and SQL are not printed")
    return result.stdout.strip()


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def uid():
    return str(uuid.uuid4())


def config_input(settings, **changes):
    return {**{field: settings[field] for field in FIELDS}, "expected_revision": settings["revision"], **changes}


def account(role, suffix):
    # Dedicated identities avoid consuming another CI journey's operator quota.
    email, password = f"mail-ui-{role}-{suffix}@example.invalid", secrets.token_urlsafe(24)
    created = request("/v1/auth/register", method="POST", body={"email": email, "password": password, "pen_name": "Mail UI " + role})
    principal = created["principal"]
    if role != "creator":
        global_roles = "ARRAY['super_admin']::text[]" if role == "super" else "ARRAY[]::text[]"
        sql("BEGIN; UPDATE platform_memberships SET roles=ARRAY['creator','operator']::text[],revision=revision+1 WHERE user_id=" + literal(principal["user_id"]) +
            "; UPDATE platform_users SET global_roles=" + global_roles + ",revision=revision+1 WHERE id=" + literal(principal["user_id"]) + "; COMMIT;")
    audience = "creator" if role == "creator" else "admin"
    cookies = []
    session = request("/v1/auth/login", method="POST", body={"email": email, "password": password, "audience": audience, "tenant_id": principal["tenant_id"]}, cookies=cookies)
    check(cookies and all(cookie["httpOnly"] for cookie in cookies), "Password login did not issue a real HttpOnly cookie")
    check(session["principal"]["audience"] == audience, "Password login returned the wrong audience")
    check(sql("SELECT email_verified_at IS NULL FROM platform_users WHERE id=" + literal(principal["user_id"])) == "t", "Ordinary password registration silently verified email")
    return session, cookies, email


def main():
    dsn = urllib.parse.urlparse(os.environ["DATABASE_DSN"])
    check(os.getenv("E2E_MAIL_UI_PG_TEST") == "1" and dsn.hostname in ("127.0.0.1", "localhost")
          and dsn.path in ("/inklore_ci", "/inklore_e2e")
          and urllib.parse.urlparse(BASE).hostname in ("127.0.0.1", "localhost"),
          "Explicit disposable local mail UI database required")
    evidence = Path(os.getenv("E2E_EVIDENCE_DIR", "/tmp/inklore-mail-ui-evidence"))
    evidence.mkdir(parents=True, exist_ok=True)
    suffix = secrets.token_hex(6)
    creator, creator_cookies, creator_email = account("creator", suffix)
    operator, operator_cookies, _ = account("operator", suffix)
    admin, admin_cookies, _ = account("super", suffix)
    original = request(SETTINGS, session=admin)
    check(not original["enabled"] and not original["has_credential"], "Mail UI journey requires unconfigured disposable mail settings")
    check(set(original) == SUMMARY_FIELDS, "Mail settings exposed an unexpected field")
    try:
        capability = request("/v1/auth/email-capabilities")
        check(capability == {"enabled": False, "code_length": 6, "expires_in": 300, "retry_after_seconds": 60}, "Disabled email capability contract drifted")
        for email in (creator_email, f"mail-ui-unknown-{suffix}@example.invalid"):
            for purpose in ("login", "reset"):
                failure = request("/v1/auth/email-challenges", 503, "POST", {"email": email, "purpose": purpose, "audience": "creator"}, key=uid())
                check(failure["code"] == "platform.dependency_unavailable", "Unconfigured mail did not fail uniformly")
        verified("Disabled mail reports its real capability, refuses known/unknown OTP requests, and preserves password registration/login")

        request(SETTINGS, 403, session=creator)
        request(SETTINGS, 403, session=operator)
        request(SETTINGS, 403, "PUT", config_input(original), operator, uid())
        request(SETTINGS + "/test", 403, "POST", {"expected_revision": original["revision"]}, operator, uid())
        verified("Creator and tenant operator cannot read, update or test global mail settings")

        body = config_input(original, sender_name="Mail UI initial")
        key = uid()
        saved = request(SETTINGS, method="PUT", body=body, session=admin, key=key)
        replay = request(SETTINGS, method="PUT", body=body, session=admin, key=key)
        check(saved == replay and saved["revision"] == original["revision"] + 1, "Configuration idempotency created another revision")
        stale = request(SETTINGS, 409, "PUT", {**body, "sender_name": "Stale update"}, admin, uid())
        check(stale["code"] == "platform.version_conflict", "Stale mail revision did not report CAS conflict")
        invalid = request(SETTINGS, 400, "PUT", config_input(saved, credential={"kind": "env", "locator": "JWT_SIGNING_KEY"}), admin, uid())
        check(invalid["code"] == "platform.validation_failed", "Arbitrary environment credential locator was accepted")
        check(request(SETTINGS, session=admin) == saved, "Failed writes changed saved settings")
        count = sql("SELECT count(*) FROM platform_audit_events WHERE actor_id=" + literal(admin["principal"]["user_id"]) + " AND action='platform.mail_settings_updated'")
        check(count == "1", "Configuration replay or failed write created an extra audit record")
        verified("Super admin CAS, replay without duplicate audit, invalid credential locator and unchanged failed-write state")

        outcome = {"checks": CHECKS, "browser": "PENDING" if os.getenv("E2E_RUN_BROWSER") == "1" else "NOT_RUN",
                   "otp_browser": "NOT_VERIFIED", "external_delivery": "NOT_VERIFIED", "mail_enabled": False}
        (evidence / "mail-ui-api.json").write_text(json.dumps(outcome, indent=2))
        if os.getenv("E2E_RUN_BROWSER") == "1":
            fixture = {"creator": {"session": creator, "cookies": creator_cookies},
                       "operator": {"session": operator, "cookies": operator_cookies},
                       "admin": {"session": admin, "cookies": admin_cookies},
                       "blocked_host": "blocked-mail-ui-" + suffix + ".example.invalid"}
            # Session credentials use an anonymous pipe, never a fixture file.
            result = subprocess.run(["node", "scripts/ci/mail_ui_browser.cjs"], input=json.dumps(fixture), text=True)
            check(result.returncode == 0, "Real mail/Auth Chromium journey failed")
            verified("Creator/Admin real capability, HttpOnly restoration, mail UI CAS and API error presentation")
            outcome["browser"] = "PASS"
        (evidence / "mail-ui-api.json").write_text(json.dumps(outcome, indent=2))
    finally:
        # Restore the originally disabled configuration through its normal CAS API.
        current = request(SETTINGS, session=admin)
        request(SETTINGS, method="PUT", body=config_input(original, expected_revision=current["revision"]), session=admin, key=uid())
    print("Mail UI API/PG acceptance passed; OTP browser and external delivery remain NOT_VERIFIED.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Do not print request bodies, sessions, cookies, passwords or SQL errors.
        print("Mail UI acceptance failed: " + type(error).__name__ + ": " + str(error)[:200], file=sys.stderr)
        sys.exit(1)
