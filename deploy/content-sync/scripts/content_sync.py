#!/usr/bin/env python3
"""Build a content snapshot and call unagi content-sync APIs."""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ALLOWED_EXT = {".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif"}
MAX_BYTES = 5 * 1024 * 1024
# Keep in sync with media.PublicCacheControl (1 day, not immutable).
CACHE_CONTROL = "public, max-age=86400"


def sha256_file(path: Path) -> tuple[str, int]:
    data = path.read_bytes()
    return hashlib.sha256(data).hexdigest(), len(data)


def collect_snapshot(root: Path, repository: str, commit: str, run_id: str) -> dict:
    articles = []
    for path in sorted((root / "articles").glob("*.md")):
        articles.append({"path": f"articles/{path.name}", "markdown": path.read_text(encoding="utf-8")})
    images = []
    image_dir = root / "images"
    if image_dir.is_dir():
        for path in sorted(image_dir.iterdir()):
            ext = path.suffix.lower()
            if not path.is_file() or ext not in ALLOWED_EXT:
                continue
            digest, size = sha256_file(path)
            if size <= 0 or size > MAX_BYTES:
                raise SystemExit(f"image {path} is empty or larger than 5 MiB")
            images.append({
                "path": f"images/{path.name}",
                "sha256": digest,
                "size": size,
                "content_type": ALLOWED_EXT[ext],
            })
    return {
        "repository": repository,
        "commit_sha": commit,
        "run_id": run_id,
        "articles": articles,
        "images": images,
    }


def sign(secret: str, method: str, path: str, timestamp: str, run_id: str, repository: str, body: bytes) -> str:
    body_hash = hashlib.sha256(body).hexdigest()
    canonical = "\n".join([method.upper(), path, timestamp, run_id, repository, body_hash])
    return hmac.new(secret.encode(), canonical.encode(), hashlib.sha256).hexdigest()


def request(base: str, path: str, secret: str, repository: str, run_id: str, body: bytes, retries: int) -> dict:
    url = base.rstrip("/") + path
    delay = 2
    last_error = None
    for attempt in range(retries + 1):
        ts = str(int(time.time()))
        sig = sign(secret, "POST", path, ts, run_id, repository, body)
        req = urllib.request.Request(url, data=body, method="POST")
        req.add_header("Content-Type", "application/json")
        req.add_header("X-Unigo-Timestamp", ts)
        req.add_header("X-Unigo-Run-Id", run_id)
        req.add_header("X-Unigo-Repository", repository)
        req.add_header("X-Unigo-Signature", "sha256=" + sig)
        try:
            with urllib.request.urlopen(req, timeout=60) as res:
                raw = res.read()
                return json.loads(raw.decode()) if raw else {}
        except urllib.error.HTTPError as err:
            detail = err.read().decode("utf-8", "replace")
            last_error = f"HTTP {err.code} {path}: {detail}"
            if err.code in {400, 401, 403, 409} or attempt == retries:
                raise SystemExit(last_error) from err
        except urllib.error.URLError as err:
            last_error = f"request {path}: {err}"
            if attempt == retries:
                raise SystemExit(last_error) from err
        time.sleep(delay)
        delay *= 2
    raise SystemExit(last_error or "request failed")


def put_upload(url: str, path: Path, content_type: str) -> None:
    data = path.read_bytes()
    req = urllib.request.Request(url, data=data, method="PUT")
    req.add_header("Content-Type", content_type)
    req.add_header("Cache-Control", CACHE_CONTROL)
    req.add_header("x-upsert", "true")
    try:
        with urllib.request.urlopen(req, timeout=60) as res:
            res.read()
    except urllib.error.HTTPError as err:
        raise SystemExit(f"upload {path.name}: HTTP {err.code} {err.read().decode('utf-8', 'replace')}") from err


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("dry-run", "sync"))
    parser.add_argument("--root", default=".")
    args = parser.parse_args()

    secret = os.environ.get("UNIGO_CONTENT_SYNC_SECRET", "").strip()
    if not secret:
        raise SystemExit("UNIGO_CONTENT_SYNC_SECRET is required")
    repository = os.environ.get("UNIGO_CONTENT_SYNC_REPOSITORY") or os.environ.get("GITHUB_REPOSITORY") or ""
    repository = repository.strip()
    if not repository:
        raise SystemExit("UNIGO_CONTENT_SYNC_REPOSITORY is required")
    base = os.environ.get("UNIGO_SITE_BASE_URL", "").strip()
    if not base:
        raise SystemExit("UNIGO_SITE_BASE_URL is required")
    commit = (os.environ.get("GITHUB_SHA") or "local").strip()
    run_id = (os.environ.get("GITHUB_RUN_ID") or f"local-{int(time.time())}").strip()

    snap = collect_snapshot(Path(args.root), repository, commit, run_id)
    body = json.dumps(snap, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")

    if args.mode == "dry-run":
        result = request(base, "/api/content-sync/dry-run", secret, repository, run_id, body, retries=2)
        print(json.dumps(result, ensure_ascii=False))
        return

    planned = request(base, "/api/content-sync/images", secret, repository, run_id, body, retries=3)
    uploads = planned.get("uploads") or []
    root = Path(args.root)
    for item in uploads:
        local = root / item["path"]
        put_upload(item["signed_url"], local, item["content_type"])
        print(f"uploaded {item['path']} -> {item['object_key']}")
    result = request(base, "/api/content-sync/sync", secret, repository, run_id, body, retries=3)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
