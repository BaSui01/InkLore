#!/usr/bin/env python3
"""Real writing-goal/confirmed-statistics acceptance on a disposable API + PG.

Requires the same isolated database used by E2E_API_URL. Never point at a shared
or production database: the upgrade check deliberately replaces a fixture total.
"""
import concurrent.futures
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import uuid
from zoneinfo import ZoneInfo

from creator_admin_agent_acceptance import call, require, check, CREATOR_ORIGIN


def sql(statement):
    return subprocess.run(["psql", "-X", "-v", "ON_ERROR_STOP=1", "-Atc", statement],
                          check=True, text=True, capture_output=True).stdout.strip()


def main():
    check(os.getenv("E2E_ALLOW_DISPOSABLE_DB") == "1", "Set E2E_ALLOW_DISPOSABLE_DB=1 for a disposable database only")
    evidence = []
    accounts = []
    for label in ("author", "other"):
        email = f"writing-{label}-{secrets.token_hex(8)}@example.invalid"
        password = secrets.token_urlsafe(24)
        account = require("/v1/auth/register", 200, method="POST", origin=CREATOR_ORIGIN,
                          body={"email": email, "password": password, "pen_name": "WritingAcceptance"})
        accounts.append((account, email, password))
    session, email, password = accounts[0]
    token, other = session["access_token"], accounts[1][0]["access_token"]

    def request(path, method="GET", body=None, expected=200, key=None, actor=token):
        result = require(path, expected, method=method, body=body, token=actor,
                         origin=CREATOR_ORIGIN, key=key or (str(uuid.uuid4()) if method != "GET" else None))
        evidence.append({"method": method, "path": path, "status": expected})
        return result

    novel = request("/v1/content/novels", "POST", {"title": "Writing acceptance", "summary": "", "category": "", "target_words": 0}, 201)
    check(novel["target_completion_date"] is None and novel["word_count"] == 0, "Optional goals/default count failed")
    path = "/v1/content/novels/" + novel["id"]
    date = datetime.datetime.now(ZoneInfo("Asia/Shanghai")).date().isoformat()

    def statistics():
        return request(path + f"/statistics?from={date}&to={date}")

    def counts(total, added, removed, operations):
        current = statistics()
        check(current["current_word_count"] == total, f"Total mismatch: {current['current_word_count']} != {total}")
        day = current["days"][0]
        check((day["added_words"], day["removed_words"], day["net_words"], day["save_count"]) ==
              (added, removed, added - removed, operations), f"Daily count mismatch: {day}")
        check(request(path)["word_count"] == total, "Novel/statistics totals diverged")
        return current

    volume = request(path + "/volumes", "POST", {"title": "Volume", "summary": ""}, 201)
    ch = request(path + "/chapters", "POST", {"volume_id": volume["id"], "title": "Chapter"}, 201)
    cp = "/v1/content/chapters/" + ch["id"]
    # Chinese, emoji, Latin, whitespace and CRLF: six non-whitespace code points.
    body = "星海 🐈\r\n A\tB。\u3000"
    saved = request(cp + "/body", "PUT", {"expected_revision": ch["revision"], "body": body})
    ch = saved["chapter"]
    check(ch["word_count"] == 6, "Unicode/whitespace count failed")
    first_version = ch["current_version_id"]
    counts(0, 0, 0, 0)

    def confirm(chapter, key=None):
        world = request("/v1/worldview/novels/" + novel["id"] + "/world-revision")
        payload = {"expected_revision": chapter["revision"], "expected_world_revision": world["revision"], "fact_proposals": []}
        key = key or str(uuid.uuid4())
        result = request(cp + "/confirm", "POST", payload, key=key)
        replay = request(cp + "/confirm", "POST", payload, key=key)
        check(result == replay, "Confirmation replay changed the result")
        return result["chapter"]

    ch = confirm(ch)
    counts(6, 6, 0, 1)
    # Re-saving identical text and confirming is a transition, not new words.
    ch = request(cp + "/body", "PUT", {"expected_revision": ch["revision"], "body": body})["chapter"]
    ch = confirm(ch)
    counts(6, 6, 0, 2)
    ch = request(cp + "/body", "PUT", {"expected_revision": ch["revision"], "body": "星海 🐈 A B。新章"})["chapter"]
    counts(6, 6, 0, 2)
    # World CAS failure must not write statistics or confirm the draft.
    request(cp + "/confirm", "POST", {"expected_revision": ch["revision"], "expected_world_revision": 9007199254740990, "fact_proposals": []}, 409)
    counts(6, 6, 0, 2)
    # Fail the statistics insert after fact commit: all chapter, world and novel
    # writes must roll back, then the same author action can be retried.
    before_world = request("/v1/worldview/novels/" + novel["id"] + "/world-revision")
    sql(f"""CREATE FUNCTION writing_acceptance_reject() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN RAISE EXCEPTION 'isolated writing failure'; END; $$;
      CREATE TRIGGER writing_acceptance_reject BEFORE INSERT ON content_writing_events
      FOR EACH ROW WHEN (NEW.chapter_id='{ch['id']}'::uuid) EXECUTE FUNCTION writing_acceptance_reject();""")
    try:
        request(cp + "/confirm", "POST", {"expected_revision": ch["revision"], "expected_world_revision": before_world["revision"], "fact_proposals": []}, 500)
        check(request("/v1/worldview/novels/" + novel["id"] + "/world-revision")["revision"] == before_world["revision"], "Failed statistics insert committed world facts")
        check(request(cp)["revision"] == ch["revision"], "Failed statistics insert committed chapter revision")
        counts(6, 6, 0, 2)
    finally:
        sql("DROP TRIGGER writing_acceptance_reject ON content_writing_events; DROP FUNCTION writing_acceptance_reject()")
    ch = confirm(ch)
    counts(8, 8, 0, 3)
    # Rollback affects working text only until explicitly confirmed.
    ch = request(cp + "/rollback", "POST", {"expected_revision": ch["revision"], "version_id": first_version})["chapter"]
    counts(8, 8, 0, 3)
    ch = confirm(ch)
    counts(6, 8, 2, 4)
    # Delete while a larger draft exists: subtract the six-word confirmed version.
    ch = request(cp + "/body", "PUT", {"expected_revision": ch["revision"], "body": "尚未确认的草稿不属于累计正文"})["chapter"]
    delete_key = str(uuid.uuid4())
    delete_path = cp + "?expected_revision=" + str(ch["revision"])
    ch = request(delete_path, "DELETE", key=delete_key)
    request(delete_path, "DELETE", key=delete_key)
    counts(0, 8, 8, 5)
    for _ in range(2):
        restore_key = str(uuid.uuid4())
        payload = {"expected_revision": ch["revision"]}
        ch = request(cp + "/restore", "POST", payload, key=restore_key)
        request(cp + "/restore", "POST", payload, key=restore_key)
        if _ == 0:
            counts(6, 14, 8, 6)
            ch = request(cp + "?expected_revision=" + str(ch["revision"]), "DELETE")
            counts(0, 14, 14, 7)
    counts(6, 20, 14, 8)

    # Two competing confirmations cannot both commit against the same revision.
    ch = request(cp + "/body", "PUT", {"expected_revision": ch["revision"], "body": "星海 🐈 A B。新章"})["chapter"]
    world = request("/v1/worldview/novels/" + novel["id"] + "/world-revision")
    payload = {"expected_revision": ch["revision"], "expected_world_revision": world["revision"], "fact_proposals": []}
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(lambda _: call(cp + "/confirm", "POST", payload, token, CREATOR_ORIGIN, str(uuid.uuid4())), range(2)))
    check(sorted(status for status, _ in results) == [200, 409], "Concurrent confirmation did not enforce CAS")
    counts(8, 22, 14, 9)

    current = request(path)
    target = {"expected_revision": current["revision"], "title": current["title"], "summary": current["summary"], "category": current["category"], "target_words": 5, "target_completion_date": "2028-02-29"}
    novel = request(path, "PUT", target)
    check(novel["target_completion_date"] == "2028-02-29" and novel["word_count"] / novel["target_words"] == 1.6, "Goal date/overcompletion failed")
    check(request(path)["target_completion_date"] == "2028-02-29", "DATE did not round-trip as YYYY-MM-DD")
    request(path, "PUT", target, 409)
    invalid = {**target, "expected_revision": novel["revision"], "target_completion_date": "2027-02-29"}
    request(path, "PUT", invalid, 422)
    cleared = request(path, "PUT", {**target, "expected_revision": novel["revision"], "target_words": 0, "target_completion_date": None})
    check(cleared["target_completion_date"] is None and cleared["target_words"] == 0, "Clearing optional goals failed")
    novel = request(path, "PUT", {**target, "expected_revision": cleared["revision"]})

    request(path + f"/statistics?from={date}&to={date}", expected=404, actor=other)
    request(path, "PUT", {**target, "expected_revision": novel["revision"]}, 404, actor=other)

    # A copy consists of drafts; its goal is copied, but no confirmed words are.
    copied = request(path + "/copy", "POST", {"expected_revision": novel["revision"], "title": "Writing copy", "include_planning": False, "include_worldview": False}, 202)
    copied_novel = request("/v1/content/novels/" + copied["target_novel_id"])
    check(copied_novel["word_count"] == 0 and copied_novel["target_completion_date"] == "2028-02-29", "Copy inherited confirmed count incorrectly")

    # Isolated persisted import preview fixture, then the real CommitImport HTTP
    # transaction. File upload/parsing is tested separately by content delivery.
    tenant, actor = session["principal"]["tenant_id"], session["principal"]["user_id"]
    job_id, item_id = str(uuid.uuid4()), str(uuid.uuid4())
    import_body = "Imported draft"
    import_hash = hashlib.sha256(import_body.encode()).hexdigest()
    sql(f"""INSERT INTO content_jobs(id,tenant_id,novel_id,actor_id,kind,status,stage,format,source_revision,expires_at)
      VALUES('{job_id}','{tenant}','{novel['id']}','{actor}','import','preview_ready','preview','txt',{novel['revision']},now()+interval '1 day');
      INSERT INTO content_import_items(id,tenant_id,job_id,ordinal,title,body,word_count,content_hash)
      VALUES('{item_id}','{tenant}','{job_id}',0,'Imported chapter','{import_body}',13,'{import_hash}');""")
    import_payload = {"expected_revision": 1, "expected_novel_revision": novel["revision"], "volume_id": volume["id"], "selected_item_ids": [item_id]}
    import_key = str(uuid.uuid4())
    committed = request("/v1/content/imports/" + job_id + "/commit", "POST", import_payload, 202, import_key)
    check(committed["result_chapter_count"] == 1, "Import transaction did not create its draft")
    request("/v1/content/imports/" + job_id + "/commit", "POST", import_payload, 202, import_key)
    imported = request("/v1/content/chapters/" + item_id)
    check(imported["status"] == "draft" and imported["confirmed_version_id"] is None, "Import unexpectedly confirmed text")
    counts(8, 22, 14, 9)
    # A legacy draft event is retained but must not be reported as confirmed work.
    sql(f"INSERT INTO content_writing_events(tenant_id,novel_id,chapter_id,version_id,actor_id,added_words,removed_words,source) VALUES('{tenant}','{novel['id']}','{ch['id']}','{first_version}','{actor}',99,0,'save')")
    counts(8, 22, 14, 9)

    # Old installation backfill: a draft save left the wrong count. Replay the
    # actual migration twice, preserving current totals and all daily events.
    sql(f"UPDATE content_novels SET word_count=12345 WHERE id='{novel['id']}'")
    migration = Path("migrations/post_r1/000004_content_writing_goals.sql")
    for _ in range(2):
        subprocess.run(["psql", "-X", "-v", "ON_ERROR_STOP=1", "-q", "-f", str(migration)], check=True, capture_output=True)
        counts(8, 22, 14, 9)
        check(sql(f"SELECT COUNT(*) FROM content_writing_events WHERE novel_id='{novel['id']}' AND chapter_revision IS NOT NULL") == "9", "Migration duplicated events")
    check(sql(f"SELECT COALESCE(SUM(v.word_count),0) FROM content_chapters c JOIN content_chapter_versions v ON v.tenant_id=c.tenant_id AND v.chapter_id=c.id AND v.id=c.confirmed_version_id WHERE c.novel_id='{novel['id']}' AND c.deleted_at IS NULL") == "8", "Confirmed body/database total mismatch")

    # Real PostgreSQL grouping on both sides of Beijing midnight, rolled back so
    # the acceptance novel and its immutable production events remain intact.
    tenant, actor, chapter_id = session["principal"]["tenant_id"], session["principal"]["user_id"], ch["id"]
    boundary = sql(f"""BEGIN;
      INSERT INTO content_writing_events (tenant_id,novel_id,chapter_id,version_id,actor_id,added_words,removed_words,source,chapter_revision,created_at)
      VALUES ('{tenant}','{novel['id']}','{chapter_id}','{first_version}','{actor}',1,0,'confirm',9000000,'2026-01-01T15:59:59Z'),
             ('{tenant}','{novel['id']}','{chapter_id}','{first_version}','{actor}',2,0,'confirm',9000001,'2026-01-01T16:00:00Z');
      SELECT (created_at AT TIME ZONE 'Asia/Shanghai')::date::text || ':' || SUM(added_words)::text
      FROM content_writing_events WHERE novel_id='{novel['id']}' AND chapter_revision IN (9000000,9000001)
      GROUP BY (created_at AT TIME ZONE 'Asia/Shanghai')::date ORDER BY (created_at AT TIME ZONE 'Asia/Shanghai')::date;
      ROLLBACK;""")
    check("2026-01-01:1" in boundary and "2026-01-02:2" in boundary, "PostgreSQL Beijing midnight grouping failed")

    output = Path(os.environ.get("E2E_EVIDENCE_DIR", "/tmp/inklore-writing-evidence"))
    output.mkdir(parents=True, exist_ok=True)
    report = {"status": "PASS", "novel_id": novel["id"], "timezone": "Asia/Shanghai", "confirmed_words": 8, "added_today": 22, "removed_today": 14,
              "confirmation_and_state_events": 9, "goal_percent": 160, "migration_replays": 2, "requests": evidence,
              "checks": ["Unicode", "draft exclusion", "idempotency", "world CAS rollback", "statistics failure transaction rollback", "chapter CAS concurrency", "rollback confirmation", "deletion/restoration", "goal/date/clear", "tenant denial", "copy drafts", "real import commit from PG preview fixture", "legacy event exclusion", "upgrade backfill", "PG midnight grouping"]}
    (output / "writing-acceptance.json").write_text(json.dumps(report, indent=2) + "\n")
    print("PASS writing goals/statistics: real API + PG, confirmed total 8, daily +22/-14, nine events, 160% goal; migration replay and isolation verified.")
    if os.getenv("E2E_WRITING_BROWSER") == "1":
        # Disposable credentials travel only through an anonymous process pipe;
        # they never enter evidence files, argv, or CI output.
        subprocess.run(["node", str(Path(__file__).with_name("content_writing_browser.cjs"))],
                       input=json.dumps({"email": email, "password": password, "novel_id": novel["id"]}),
                       text=True, check=True)


if __name__ == "__main__":
    main()
