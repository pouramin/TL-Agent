#!/usr/bin/env python3
"""Verify that TL-Agent routes Kilo's production HttpApi to the selected project."""

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
    selected = local.get("project")
    if not isinstance(selected, str):
        print(f"invalid local project response: {selected!r}", file=sys.stderr)
        return 1

    quoted = urllib.parse.quote(selected, safe="")
    routed = get_json(base, f"/kilo/path?directory={quoted}")
    if isinstance(routed, dict) and isinstance(routed.get("data"), dict):
        routed = routed["data"]
    directory = routed.get("directory") if isinstance(routed, dict) else None
    if not isinstance(directory, str):
        print(f"invalid /path response: {routed!r}", file=sys.stderr)
        return 1
    if canonical(selected) != canonical(directory):
        print(f"project routing mismatch: selected={selected!r} routed={directory!r}", file=sys.stderr)
        return 1

    print(json.dumps({"ok": True, "selected": selected, "routed": directory}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
