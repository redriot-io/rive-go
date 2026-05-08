#!/usr/bin/env python3
"""Download dev/defs JSON files from rive-runtime at a pinned commit."""
import json
import os
import sys
import time
import urllib.request
import urllib.error

COMMIT = "e031209510932a392451786322400afbffd94823"
REPO = "rive-app/rive-runtime"
BASE_RAW = f"https://raw.githubusercontent.com/{REPO}/{COMMIT}"
BASE_API = f"https://api.github.com/repos/{REPO}/git/trees/{COMMIT}?recursive=1"
OUT_DIR = "internal/schema/defs"


def fetch_json(url):
    req = urllib.request.Request(url, headers={"User-Agent": "rivegen-fetch/1.0"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def fetch_raw(url, retries=3):
    for attempt in range(retries):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "rivegen-fetch/1.0"})
            with urllib.request.urlopen(req, timeout=30) as resp:
                return resp.read()
        except urllib.error.URLError as e:
            if attempt < retries - 1:
                time.sleep(2 ** attempt)
                continue
            raise


def main():
    print(f"Fetching tree for commit {COMMIT[:12]}...")
    tree_data = fetch_json(BASE_API)

    defs_files = [
        item for item in tree_data["tree"]
        if item["type"] == "blob" and item["path"].startswith("dev/defs/") and item["path"].endswith(".json")
    ]
    print(f"Found {len(defs_files)} JSON files in dev/defs/")

    os.makedirs(OUT_DIR, exist_ok=True)

    for i, item in enumerate(defs_files):
        rel_path = item["path"][len("dev/defs/"):]  # strip "dev/defs/" prefix
        out_path = os.path.join(OUT_DIR, rel_path)
        os.makedirs(os.path.dirname(out_path), exist_ok=True)

        raw_url = f"{BASE_RAW}/{item['path']}"
        content = fetch_raw(raw_url)
        with open(out_path, "wb") as f:
            f.write(content)

        if (i + 1) % 50 == 0 or (i + 1) == len(defs_files):
            print(f"  [{i+1}/{len(defs_files)}] {rel_path}")

    print("Done.")


if __name__ == "__main__":
    main()
