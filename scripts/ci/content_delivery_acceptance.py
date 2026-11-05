#!/usr/bin/env python3
"""Validate real content files against an isolated API/PostgreSQL deployment."""
import concurrent.futures
import hashlib
import io
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.request
import uuid
import zipfile
import xml.etree.ElementTree as ET

BASE = os.getenv("E2E_API_URL", "http://127.0.0.1:18080").rstrip("/")
ORIGIN = "http://127.0.0.1:3000"


def request(path, method="GET", body=None, token=None, key=None, expected=200, binary=False):
    headers = {"Origin": ORIGIN, "X-Request-ID": str(uuid.uuid4())}
    if token:
        headers["Authorization"] = "Bearer " + token
    if method != "GET":
        headers["Idempotency-Key"] = key or str(uuid.uuid4())
    if body is not None:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, method=method, headers=headers,
                                 data=json.dumps(body).encode() if body is not None else None)
    try:
        response = urllib.request.urlopen(req, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        payload = response.read()
        accepted = (expected,) if isinstance(expected, int) else expected
        assert response.status in accepted, f"{method} {path}: expected {expected}, got {response.status}: {payload[:250]!r}"
        if binary:
            return payload, dict(response.headers)
        result = json.loads(payload)
        return result.get("data", result)


def replay_export(path, payload, token, key):
    # A concurrent request may still hold the idempotency lock; recover the same
    # operation instead of changing its key or requiring an immediate replay.
    for attempt in range(5):
        result = request(path, "POST", payload, token, key, (202, 409))
        if "id" in result:
            return result
        error = result.get("error", result)
        assert error["code"] == "platform.operation_in_progress", result
        time.sleep(min(2, max(0.1, error.get("details", {}).get("retry_after_seconds", 1))))
    raise AssertionError("export did not recover after the idempotency lock was released")


def main():
    accounts = []
    for label in ("author", "other"):
        email = f"delivery-{label}-{secrets.token_hex(8)}@example.invalid"
        password = secrets.token_urlsafe(24)
        session = request("/v1/auth/register", "POST", {"email": email, "password": password, "pen_name": "DeliveryAuthor"})
        accounts.append((session, email, password))
    session, email, password = accounts[0]
    token = session["access_token"]
    other = accounts[1][0]["access_token"]
    novel = request("/v1/content/novels", "POST", {"title": "星海归途", "summary": "隔离导出验收", "category": "fantasy", "target_words": 0}, token, expected=201)
    novel_path = "/v1/content/novels/" + novel["id"]
    volume = request(novel_path + "/volumes", "POST", {"title": "第一卷", "summary": ""}, token, expected=201)
    chapter = request(novel_path + "/chapters", "POST", {"volume_id": volume["id"], "title": "第一章 & 启程"}, token, expected=201)
    chapter_path = "/v1/content/chapters/" + chapter["id"]
    body = "风吹过山谷。\nThe author decides the story. <正文>"
    saved = request(chapter_path + "/body", "PUT", {"expected_revision": chapter["revision"], "body": body}, token)
    chapter = saved["chapter"]
    world = request("/v1/worldview/novels/" + novel["id"] + "/world-revision", token=token)
    confirmed = request(chapter_path + "/confirm", "POST", {"expected_revision": chapter["revision"], "expected_world_revision": world["revision"], "fact_proposals": []}, token)
    assert confirmed["chapter"]["status"] == "confirmed"
    novel = request(novel_path, token=token)
    jobs = []
    for fmt, suffix in (("txt", ".txt"), ("markdown", ".md"), ("docx", ".docx"), ("epub", ".epub")):
        payload = {"expected_revision": novel["revision"], "format": fmt, "include_drafts": False, "volume_ids": [], "review_ticket": None}
        key = str(uuid.uuid4())
        job = request(novel_path + "/exports", "POST", payload, token, key, 202)
        assert job["status"] == "succeeded" and job["output_file_id"] and job["result_chapter_count"] == 1
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            replays = list(pool.map(lambda _: replay_export(novel_path + "/exports", payload, token, key), range(2)))
        assert all(replay["id"] == job["id"] and replay["output_file_id"] == job["output_file_id"] for replay in replays)
        file_path = "/v1/platform/files/" + job["output_file_id"]
        descriptor = request(file_path, token=token)
        content, headers = request(file_path + "/download", token=token, binary=True)
        assert descriptor["filename"].endswith(suffix)
        assert descriptor["size_bytes"] == len(content) and descriptor["sha256"] == hashlib.sha256(content).hexdigest()
        assert "attachment" in headers.get("Content-Disposition", "")
        if fmt in ("txt", "markdown"):
            assert body in content.decode("utf-8")
        else:
            archive = zipfile.ZipFile(io.BytesIO(content))
            assert archive.testzip() is None
            for name in archive.namelist():
                if name.endswith((".xml", ".opf", ".xhtml", ".ncx", ".rels")):
                    ET.fromstring(archive.read(name))
            if fmt == "docx":
                assert "风吹过山谷" in archive.read("word/document.xml").decode()
            else:
                assert archive.namelist()[0] == "mimetype" and archive.getinfo("mimetype").compress_type == zipfile.ZIP_STORED
                assert archive.read("mimetype") == b"application/epub+zip"
                assert "风吹过山谷" in archive.read("OEBPS/chapter_1.xhtml").decode()
        request(file_path, token=other, expected=404)
        request(file_path + "/download", token=other, expected=404)
        request("/v1/content/jobs/" + job["id"], token=other, expected=404)
        jobs.append(job)
        print(f"PASS {fmt}: real file bytes, format structure, SHA256, idempotent concurrent replay and tenant denial")
    listed = request(novel_path + "/jobs", token=token)
    assert len(listed["items"]) == 4
    bad = {"expected_revision": 1, "format": "txt", "include_drafts": False, "volume_ids": [], "review_ticket": None}
    request(novel_path + "/exports", "POST", bad, token, expected=409)
    bad["expected_revision"] = novel["revision"]
    bad["review_ticket"] = str(uuid.uuid4())
    request(novel_path + "/exports", "POST", bad, token, expected=409)
    request(novel_path + "/imports", "POST", {"expected_revision": novel["revision"], "format": "txt", "file_id": jobs[0]["output_file_id"]}, token, expected=404)
    assert len(request(novel_path + "/jobs", token=token)["items"]) == 4
    query = f"SELECT count(*) FROM content_jobs j JOIN platform_files f ON f.id=j.output_file_id AND f.tenant_id=j.tenant_id WHERE j.novel_id='{novel['id']}' AND f.owner_id=j.actor_id AND f.sha256=encode(digest(f.payload,'sha256'),'hex')"
    actual = subprocess.check_output(["psql", "-X", "-Atc", query], text=True).strip()
    assert actual == "4", "database output/job ownership or payload integrity mismatch"
    print("PASS database: exactly four persisted output files, matching job/owner/tenant/digest; rejected requests had no side effects")

    if os.getenv("E2E_BROWSER") == "true":
        browser_check(novel["id"], email, password)


def browser_check(novel_id, email, password):
    evidence = Path(os.environ.get("E2E_EVIDENCE_DIR", "/tmp"))
    evidence.mkdir(parents=True, exist_ok=True)
    subprocess.run(["node", "scripts/ci/content_delivery_browser.cjs"], check=True, text=True,
                   input=json.dumps({"novel_id": novel_id, "email": email, "password": password}))
    with zipfile.ZipFile(evidence / "browser-export.epub") as archive:
        assert archive.read("mimetype") == b"application/epub+zip"
        assert "风吹过山谷" in archive.read("OEBPS/chapter_1.xhtml").decode()
    print("PASS Chromium: real EPUB download, lost-response recovery, refreshed history and 390px dialog")


if __name__ == "__main__":
    main()
