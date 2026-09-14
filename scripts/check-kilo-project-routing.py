#!/usr/bin/env python3
"""Verify that TL-Agent routes Kilo requests to the selected project, not server cwd."""

from __future__ import annotations

import json
import os
import sys
import urllib.request


def get_json(base: str, path: str):
    req = urllib.request.Request(base.rstrip("/") + path, headers={"Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.loads(response.read())


def canonical(path: str) -> str:
    return os.path.normcase(os.path.realpath(path))


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: check-kilo-project-routing.py <launcher-base-url>", file=sys.stderr)
        return 2

    base = sys.argv[1]
    local = get_json(base, "/local/status")
    location = get_json(base, "/kilo/api/location")

    selected = local.get("project")
    routed = location.get("directory")
    if not isinstance(selected, str) or not isinstance(routed, str):
        print(f"invalid project/location response: selected={selected!r} routed={routed!r}", file=sys.stderr)
        return 1
    if canonical(selected) != canonical(routed):
        print(f"project routing mismatch: selected={selected!r} routed={routed!r}", file=sys.stderr)
        return 1

    print(json.dumps({"ok": True, "selected": selected, "routed": routed}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
