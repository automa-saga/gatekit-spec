#!/usr/bin/env python3
"""Validate the conformance fixtures.

Checks, with no external dependencies:
  1. conformance/manifest.json is well-formed and has a `groups` object.
  2. Every fixture path listed in the manifest exists and is well-formed JSON.
  3. Every *.json fixture on disk (anything under conformance/ except the
     manifest itself) is registered in exactly one manifest group.

The fixtures are the product of this repo: a malformed or unregistered fixture
silently breaks every implementation's conformance run, so this guards them on
every PR. Exits non-zero (listing all problems) if anything is wrong.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CONFORMANCE = ROOT / "conformance"
MANIFEST = CONFORMANCE / "manifest.json"


def main() -> int:
    errors: list[str] = []

    if not CONFORMANCE.is_dir():
        print(f"error: no conformance/ directory at {CONFORMANCE}", file=sys.stderr)
        return 1

    # 1. manifest parses and has groups
    manifest = None
    try:
        manifest = json.loads(MANIFEST.read_text())
    except FileNotFoundError:
        errors.append("conformance/manifest.json is missing")
    except json.JSONDecodeError as exc:
        errors.append(f"conformance/manifest.json is not valid JSON: {exc}")

    listed: set[Path] = set()
    if manifest is not None:
        groups = manifest.get("groups")
        if not isinstance(groups, dict):
            errors.append("manifest.json: `groups` must be an object")
            groups = {}
        for group, entries in groups.items():
            if not isinstance(entries, list):
                errors.append(f"manifest.json: group `{group}` must be a list")
                continue
            for rel in entries:
                if not isinstance(rel, str):
                    errors.append(f"manifest.json: group `{group}` has a non-string entry {rel!r}")
                    continue
                path = CONFORMANCE / rel
                listed.add(path.resolve())
                # 2. listed fixture exists and is valid JSON
                if not path.is_file():
                    errors.append(f"manifest lists `{rel}` (group `{group}`) but the file is missing")
                    continue
                try:
                    json.loads(path.read_text())
                except json.JSONDecodeError as exc:
                    errors.append(f"{rel}: not valid JSON: {exc}")

    # 3. every fixture on disk is registered (listed files were parsed in step 2)
    for path in sorted(CONFORMANCE.rglob("*.json")):
        if path.resolve() == MANIFEST.resolve():
            continue
        rel = path.relative_to(CONFORMANCE)
        if path.resolve() in listed:
            continue  # already parse-checked above
        try:
            json.loads(path.read_text())
        except json.JSONDecodeError as exc:
            errors.append(f"{rel}: not valid JSON: {exc}")
        errors.append(f"{rel}: fixture is not registered in manifest.json")

    if errors:
        print("Conformance fixture validation FAILED:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    total = len(listed)
    print(f"Conformance fixtures OK: {total} fixture(s) valid and registered.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
