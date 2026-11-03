#!/usr/bin/env python3
"""用仓库迁移身份和内容核对隔离 CI 数据库，避免冻结迁移数量。"""

import hashlib
import json
import re
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[2]
expected = []
for file in (root / "migrations" / "r1").iterdir():
    match = re.fullmatch(r"([0-9]+)_(.+)\.sql", file.name)
    if not file.is_file() or not match:
        continue
    # 与 database.LoadMigrations 一致，只规范化 CRLF，保留历史 SQL 原文。
    content = file.read_bytes().replace(b"\r\n", b"\n")
    expected.append({
        "version": match[1],
        "name": match[2],
        "checksum": hashlib.sha256(content).hexdigest(),
    })
expected.sort(key=lambda row: row["version"])
if not expected or len({row["version"] for row in expected}) != len(expected):
    raise SystemExit("Repository migration stream is empty or contains duplicate versions")

actual = json.load(sys.stdin)
if actual != expected:
    raise SystemExit(
        "Applied migration history differs from repository versions, names or checksums"
    )
print(len(expected))
