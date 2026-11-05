#!/usr/bin/env python3
"""Disposable PG/API checks for #106, with optional real Chromium.

Paused questions are explicitly seeded test state in a fresh local database.
No HTTP result, UI data, provider response, or model success is mocked. These
checks do not claim live-provider generation; that requires configured keys.
"""
import hashlib
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


def check(value, message):
    if not value:
        raise AssertionError(message)


def verified(message):
    CHECKS.append(message)
    print("PASS " + message, flush=True)


def request(path, expected=200, method="GET", body=None, session=None, key=None, cookies=None):
    headers = {"Accept": "application/json"}
    if session:
        headers.update(Authorization="Bearer " + session["access_token"], Origin=ORIGINS[session["principal"]["audience"]])
    if key:
        headers["Idempotency-Key"] = key
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body, ensure_ascii=False).encode()
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
    allowed = expected if isinstance(expected, tuple) else (expected,)
    check(status in allowed, f"{method} {path.split('?')[0]} returned {status}, expected {expected}, code={payload.get('code', 'unknown')}")
    if status >= 300:
        return payload
    check(payload.get("code") == "ok" and "data" in payload, "Invalid API success envelope")
    return payload["data"]


def sql(statement):
    result = subprocess.run(["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1"], input=statement, text=True, capture_output=True)
    if result.returncode:
        raise AssertionError("Isolated Agent UI fixture SQL failed; no SQL or credentials printed")
    return result.stdout.strip()


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def uid():
    return str(uuid.uuid4())


def seed_question(base_run, schema, options, summary):
    run_id, job_id, node_id, question_id = uid(), uid(), uid(), uid()
    values = {"run": literal(run_id), "job": literal(job_id), "node": literal(node_id), "question": literal(question_id), "base": literal(base_run), "schema": literal(json.dumps(schema)), "options": literal(json.dumps(options)), "summary": literal(summary), "checkpoint": literal(json.dumps({"observation": "", "question_id": question_id}))}
    sql("""BEGIN;
INSERT INTO platform_jobs(id,tenant_id,owner_id,domain,job_kind,resource_id,execution_key,state,available_at)
SELECT {job},tenant_id,owner_user_id,'agent','run',{run},{run},'retry_wait',clock_timestamp()+interval '12 minutes' FROM agent_runs WHERE id={base};
INSERT INTO agent_runs(id,tenant_id,owner_user_id,platform_job_id,kind,task_kind,task_key,resource_type,resource_id,novel_id,chapter_id,source_revision,source_hash,instruction,definition_version_id,policy_version_id,status,deadline_at,started_at,next_seq)
SELECT {run},tenant_id,owner_user_id,{job},kind,task_kind,task_key,resource_type,resource_id,novel_id,chapter_id,source_revision,source_hash,'private fixture instruction must never reach admin',definition_version_id,policy_version_id,'waiting_clarification',clock_timestamp()+interval '20 minutes',clock_timestamp(),2 FROM agent_runs WHERE id={base};
INSERT INTO agent_run_nodes(id,tenant_id,run_id,node_key,role,definition_version_id,status,phase,checkpoint)
SELECT {node},tenant_id,{run},'root','writer',definition_version_id,'waiting_clarification','think',{checkpoint}::jsonb FROM agent_runs WHERE id={run};
INSERT INTO agent_run_questions(id,tenant_id,run_id,node_id,owner_user_id,policy_version_id,definition_version_id,source_revision,source_hash,title_key,help_key,context_summary,response_schema,options,expires_at)
SELECT {question},tenant_id,id,{node},owner_user_id,policy_version_id,definition_version_id,source_revision,source_hash,'agentQuestion.title','agentQuestion.help',{summary},{schema}::jsonb,{options}::jsonb,clock_timestamp()+interval '12 minutes' FROM agent_runs WHERE id={run};
INSERT INTO agent_run_events(tenant_id,run_id,seq,event_name,payload)
SELECT tenant_id,id,1,'question.required',jsonb_build_object('question_id',{question}::text,'question_version',1,'node_id',{node}::text) FROM agent_runs WHERE id={run};
COMMIT;""".format(**values))
    return {"run_id": run_id, "question_id": question_id, "schema": schema, "summary": summary}


def main():
    dsn = urllib.parse.urlparse(os.environ["DATABASE_DSN"])
    check(os.getenv("E2E_AGENT_UI_PG_TEST") == "1" and dsn.hostname in ("127.0.0.1", "localhost") and dsn.path in ("/inklore_ci", "/inklore_e2e") and urllib.parse.urlparse(BASE).hostname in ("127.0.0.1", "localhost"), "Explicit disposable local Agent UI database required")
    suffix = secrets.token_hex(6)
    password, admin_password = secrets.token_urlsafe(24), secrets.token_urlsafe(24)
    email, admin_email = f"ui-{suffix}@example.invalid", f"ui-admin-{suffix}@example.invalid"
    if os.getenv("E2E_ADMIN_EMAIL") and os.getenv("E2E_ADMIN_PASSWORD"):
        admin_email, admin_password = os.environ["E2E_ADMIN_EMAIL"], os.environ["E2E_ADMIN_PASSWORD"]
    else:
        result = subprocess.run(["go", "run", "./cmd/admin", "bootstrap", "-tenant-name", "Agent UI fixture", "-email", admin_email, "-display-name", "UI Operator", "-username", "root"], input=admin_password + "\n", text=True, capture_output=True)
        check(result.returncode == 0, "Disposable operator bootstrap failed")
    creator = request("/v1/auth/register", method="POST", body={"email": email, "password": password, "pen_name": "UI Author"})
    other = request("/v1/auth/register", method="POST", body={"email": f"ui-other-{suffix}@example.invalid", "password": secrets.token_urlsafe(24), "pen_name": "Other Author"})
    tenant = creator["principal"]["tenant_id"]
    admin_login = {"email": admin_email, "password": admin_password, "audience": "admin", "tenant_id": tenant}
    admin_cookies = []
    admin = request("/v1/auth/login", method="POST", body=admin_login, cookies=admin_cookies)
    other_admin = request("/v1/auth/login", method="POST", body={**admin_login, "tenant_id": other["principal"]["tenant_id"]})
    provider = request("/v1/llm/providers", 201, "POST", {"code": "ui_" + suffix, "title": "UI acceptance", "protocol": "openai", "enabled": True}, admin, uid())
    model = request("/v1/llm/models", 201, "POST", {"provider_id": provider["id"], "upstream_name": "fixture-no-dispatch", "title": "UI acceptance model", "capabilities": ["text", "tools"], "context_window": 8192, "max_output_tokens": 4096, "embedding_dimensions": None, "enabled": True}, admin, uid())
    policy_body = {"task_key": "ui_" + suffix, "task_kinds": ["chapter_draft", "chapter_rewrite"], "title": "UI Task Policy", "enabled": True, "primary_model_id": model["id"], "fallback_model_ids": [], "max_output_tokens": 512, "temperature": 0.2, "timeout_seconds": 30, "max_retries": 0, "required_capabilities": ["text", "tools"], "output_mode": "text"}
    policy = request("/v1/llm/task-policies", 201, "POST", policy_body, admin, uid())
    definition = request("/v1/agents/definitions", 201, "POST", {"definition_key": "ui_" + suffix, "title": "UI Author Agent", "enabled": True, "system_instruction": "private fixture system prompt", "tool_version_ids": [], "task_kinds": ["chapter_draft", "chapter_rewrite"]}, admin, uid())
    request("/v1/llm/task-policies", 201, "POST", {**policy_body, "task_key": "ui_second_" + suffix, "title": "UI Secondary Policy"}, admin, uid())
    request("/v1/agents/definitions", 201, "POST", {"definition_key": "ui_second_" + suffix, "title": "UI Second Agent", "enabled": True, "system_instruction": "Another private fixture system prompt", "tool_version_ids": [], "task_kinds": ["chapter_draft"]}, admin, uid())
    for index in range(2):
        request("/v1/agents/workflows", 201, "POST", {"workflow_key": "ui_" + str(index) + "_" + suffix, "title": "UI workflow " + str(index), "enabled": True, "manifest": {"nodes": [{"node_key": "writer", "role": "writer", "depends_on": [], "required": True, "definition_version_id": definition["current_version_id"]}]}}, admin, uid())
    for kind in ("task-policies", "definitions", "workflows"):
        path = "/v1/agents/catalog/" + kind
        first_page = request(path + "?limit=1", session=creator)
        second_page = request(path + "?limit=1&cursor=" + urllib.parse.quote(first_page["next_cursor"]), session=creator)
        identifier = "policy_id" if kind == "task-policies" else "id"
        check(first_page["items"][0][identifier] != second_page["items"][0][identifier], "Creator catalog repeated the first page")
        request(path + "?limit=1&cursor=" + urllib.parse.quote(first_page["next_cursor"]), 400, session=other)
    verified("Signed Creator policy/definition/workflow cursors advance and reject another tenant")
    for path in ("task-policies", "providers", "channels", "keys", "usage", "pricing"):
        request("/v1/llm/" + path, 403, session=creator)
    catalog = request("/v1/agents/catalog/task-policies", session=creator)["items"]
    allowed = {"policy_id", "policy_version_id", "task_key", "title", "version", "task_kinds", "input_schema", "budget"}
    check(catalog and all(set(item) == allowed for item in catalog), "Creator catalog leaked private policy fields")
    check(catalog[0]["budget"] == {"max_output_tokens": 512, "timeout_seconds": 30}, "Catalog budget is not real")
    check(request("/v1/agents/catalog/task-policies", session=other)["items"] == [], "Policy catalog leaked across tenants")
    verified("Creator policy allowlist, real budget, Admin-only LLM metadata and tenant isolation")

    novel = request("/v1/content/novels", 201, "POST", {"title": "UI Agent Novel", "summary": "Disposable UI acceptance", "category": "fantasy", "target_words": 10000}, creator, uid())
    volume = request(f"/v1/content/novels/{novel['id']}/volumes", 201, "POST", {"title": "Volume", "summary": ""}, creator, uid())
    # Non-first chapter proves the original Run pointer restores its actual source.
    request(f"/v1/content/novels/{novel['id']}/chapters", 201, "POST", {"title": "First chapter", "volume_id": volume["id"]}, creator, uid())
    chapter = request(f"/v1/content/novels/{novel['id']}/chapters", 201, "POST", {"title": "Question source", "volume_id": volume["id"]}, creator, uid())
    original = "作者保留的正文。The agent must not change it."
    chapter = request(f"/v1/content/chapters/{chapter['id']}/body", method="PUT", body={"expected_revision": chapter["revision"], "body": original}, session=creator, key=uid())["chapter"]
    body = {"task_kind": "chapter_draft", "task_policy_version_id": policy["current_version_id"], "resource": {"resource_id": chapter["id"], "resource_type": "chapter", "novel_id": novel["id"], "source_revision": chapter["revision"], "source_hash": chapter["content_hash"]}, "definition_id": definition["id"], "instruction": "UI acceptance instruction"}
    create_key = uid()
    run = request("/v1/agents/runs", 202, "POST", body, creator, create_key)
    next_policy = request(f"/v1/llm/task-policies/{policy['id']}", method="PUT", body={**policy_body, "expected_revision": policy["revision"], "max_output_tokens": 768}, session=admin, key=uid())
    check(next_policy["version"] == policy["version"] + 1 and next_policy["current_version_id"] != policy["current_version_id"], "Policy publish did not create immutable version")
    replay = request("/v1/agents/runs", 202, "POST", body, creator, create_key)
    check(replay["id"] == run["id"] and replay["policy_version_id"] == policy["current_version_id"], "Create replay rebound frozen policy")
    request("/v1/agents/runs", 409, "POST", body, creator, uid())
    request("/v1/agents/runs", 400, "POST", {**body, "task_policy_version_id": next_policy["current_version_id"], "task_key": policy["task_key"]}, creator, uid())
    request("/v1/agents/runs", 400, "POST", {**body, "task_policy_version_id": next_policy["current_version_id"], "model_id": model["id"]}, creator, uid())
    request("/v1/agents/runs/" + run["id"], 404, session=other)
    fresh = request("/v1/agents/runs/" + run["id"], session=creator)
    if fresh["status"] not in ("succeeded", "failed", "cancelled"):
        request(f"/v1/agents/runs/{run['id']}/cancel", method="POST", body={"expected_revision": fresh["revision"]}, session=creator, key=uid())
    verified("Real CreateRun, frozen version, changed-version CAS, idempotent replay, override rejection and cancellation")

    question = seed_question(run["id"], {"type": "string", "minLength": 1, "maxLength": 200}, [], "Persisted text clarification")
    question_path = f"/v1/agents/runs/{question['run_id']}/questions/{question['question_id']}"
    pending = request("/v1/agents/runs/" + question["run_id"], session=creator)
    check(pending["status"] == "waiting_clarification" and pending["pending_questions"][0]["id"] == question["question_id"], "Original run did not recover its persisted question")
    request(question_path, 404, session=other)
    request(question_path, 403, session=admin)
    answer_key = uid()
    answer = {"expected_revision": 1, "question_version": 1, "answer": "作者确认的方向"}
    first = request(question_path + "/answer", method="POST", body=answer, session=creator, key=answer_key)
    repeated = request(question_path + "/answer", method="POST", body=answer, session=creator, key=answer_key)
    check(first == repeated and first["status"] == "answered" and "answer" not in first, "Answer replay did not preserve safe CAS result")
    request(question_path + "/answer", 409, "POST", {**answer, "answer": "Different answer"}, creator, uid())
    check(request("/v1/agents/runs/" + question["run_id"], session=creator)["pending_questions"] == [], "Answered question was not removed from original run")
    check(request(f"/v1/content/chapters/{chapter['id']}/body", session=creator)["body"] == original, "Answer modified authoritative chapter")
    verified("Persisted Question GET/Answer, same-key replay, conflicting answer rejection, original Run and unchanged author body")

    keys = {"id", "revision", "created_at", "updated_at", "kind", "task_kind", "status", "node_count", "duration_ms", "cost_micros", "error_code"}
    audit = request("/v1/agents/audit/runs/" + question["run_id"], session=admin)
    check(set(audit) == keys and audit["node_count"] == 1 and audit["cost_micros"] == 0, "Admin audit projection is not exact or cost was fabricated")
    request("/v1/agents/audit/runs/" + question["run_id"], 404, session=other_admin)
    request("/v1/agents/audit/runs", 403, session=creator)
    for path, session in [("/v1/agents/runs", creator), ("/v1/agents/audit/runs", admin)]:
        page1 = request(path + "?limit=1", session=session)
        check(page1["has_more"] and page1["next_cursor"], "Expected real second page")
        page2 = request(path + "?limit=1&cursor=" + urllib.parse.quote(page1["next_cursor"]), session=session)
        check(page1["items"][0]["id"] != page2["items"][0]["id"], "Run cursor repeated the first page")
        request(path + "?limit=1&cursor=" + urllib.parse.quote(page1["next_cursor"]), 400, session=other if session is creator else other_admin)
    verified("Exact Admin audit allowlist, actual zero receipts, tenant rejection and signed paginated original-run lookup")

    # The answered API-only fixture has no model-dispatch configuration. Cancel
    # it through the actual owner endpoint before the server stops; otherwise a
    # later shared-queue worker test could legitimately claim this leftover job.
    for _ in range(3):
        pending = request("/v1/agents/runs/" + question["run_id"], session=creator)
        if pending["status"] in ("succeeded", "failed", "cancelled"):
            break
        result = request(f"/v1/agents/runs/{question['run_id']}/cancel", expected=(200, 409), method="POST", body={"expected_revision": pending["revision"]}, session=creator, key=uid())
        if "code" in result:
            # Only an actual CAS race permits another read and cancellation.
            check(result["code"] == "agent.version_conflict", "Unexpected cancellation conflict")
    check(request("/v1/agents/runs/" + question["run_id"], session=creator)["status"] in ("succeeded", "failed", "cancelled"), "API fixture did not reach an actual terminal state")

    fixtures = [seed_question(run["id"], {"type": "string", "minLength": 1, "maxLength": 200}, [], "Text question fixture"), seed_question(run["id"], {"type": "string", "enum": ["first", "third"]}, [{"value": "first", "label_key": "test.first"}, {"value": "third", "label_key": "test.third"}], "Single choice fixture"), seed_question(run["id"], {"type": "array", "uniqueItems": True, "items": {"type": "string", "enum": ["mystery", "family", "travel"]}, "minItems": 1, "maxItems": 2}, [{"value": value, "label_key": "test." + value} for value in ("mystery", "family", "travel")], "Multiple choice fixture")]
    sql("UPDATE agent_run_events SET seq=3 WHERE run_id=" + literal(fixtures[2]["run_id"]) + "; UPDATE agent_runs SET retained_from_seq=3,next_seq=4 WHERE id=" + literal(fixtures[2]["run_id"]) + ";")
    evidence = Path(os.getenv("E2E_EVIDENCE_DIR", "/tmp/inklore-agent-ui-evidence"))
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "agent-ui-api.json").write_text(json.dumps({"checks": CHECKS, "model": "NOT_VERIFIED: no live provider credentials", "question_source": "explicit paused PG fixture", "browser": "PENDING" if os.getenv("E2E_RUN_BROWSER") == "1" else "NOT_RUN"}, ensure_ascii=False, indent=2))
    if os.getenv("E2E_RUN_BROWSER") == "1":
        # Reuse this journey's real HttpOnly session: preceding CI journeys and
        # tenant-isolation checks already consume the operator's login quota.
        fixture = {"creator": {"email": email, "password": password, "audience": "creator"}, "admin": admin_login, "admin_session": {"access_token": admin["access_token"], "cookies": admin_cookies}, "novel_id": novel["id"], "chapter_id": chapter["id"], "policy_id": policy["id"], "questions": fixtures, "source_body": original}
        # Generated fixture secrets are passed through an anonymous pipe only.
        completed = subprocess.run(["node", "scripts/ci/agent_question_ui_browser.cjs"], input=json.dumps(fixture), text=True)
        check(completed.returncode == 0, "Real Chromium Agent UI acceptance failed")
        verified("Real Creator/Admin Chromium question and recovery behavior")
    evidence = Path(os.getenv("E2E_EVIDENCE_DIR", "/tmp/inklore-agent-ui-evidence"))
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "agent-ui-api.json").write_text(json.dumps({"checks": CHECKS, "model": "NOT_VERIFIED: no live provider credentials", "question_source": "explicit paused PG fixture", "browser": "PASS" if os.getenv("E2E_RUN_BROWSER") == "1" else "NOT_RUN"}, ensure_ascii=False, indent=2))
    print("Agent UI API/PG acceptance passed; live model remains NOT_VERIFIED.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Agent UI acceptance failed: " + type(error).__name__ + ": " + str(error)[:240], file=sys.stderr)
        sys.exit(1)
