#!/usr/bin/env python3
"""Runtime contract check for the Kilo v7.6.2 product HttpApi used by official clients."""

from __future__ import annotations

import json
import sys
import urllib.parse
import urllib.request


class ContractError(RuntimeError):
    pass


def require(condition: bool, message: str):
    if not condition:
        raise ContractError(message)


def unwrap(value):
    if isinstance(value, dict) and "data" in value:
        return value["data"]
    return value


def request(base: str, path: str, method: str = "GET", payload=None, timeout=20):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base.rstrip("/") + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as res:
        raw = res.read()
        if not raw:
            return None
        ctype = res.headers.get("Content-Type", "")
        require("json" in ctype, f"{method} {path}: expected JSON, got {ctype!r}")
        return json.loads(raw)


def directory_query(project: str, extra=None):
    query = {"directory": project}
    if extra:
        query.update(extra)
    return urllib.parse.urlencode(query)


def first_global_event(base: str, project: str):
    path = "/kilo/global/event?" + directory_query(project)
    req = urllib.request.Request(base.rstrip("/") + path, headers={"Accept": "text/event-stream"})
    with urllib.request.urlopen(req, timeout=10) as res:
        require("text/event-stream" in (res.headers.get("Content-Type") or ""), "global/event is not SSE")
        data_lines = []
        for _ in range(150):
            raw = res.readline()
            if not raw:
                break
            line = raw.decode("utf-8", errors="replace").rstrip("\r\n")
            if not line:
                if data_lines:
                    return json.loads("\n".join(data_lines))
                continue
            if line.startswith("data:"):
                data_lines.append(line[5:].lstrip())
    raise ContractError("global/event did not yield an SSE event")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: check-kilo-product-contract.py <launcher-base-url>", file=sys.stderr)
        return 2
    base = sys.argv[1].rstrip("/")

    local = request(base, "/local/status")
    require(isinstance(local, dict) and isinstance(local.get("project"), str), "local/status.project missing")
    project = local["project"]
    query = directory_query(project)

    health = unwrap(request(base, "/kilo/global/health"))
    require(isinstance(health, dict) and health.get("healthy") is True, f"global health mismatch: {health!r}")

    path = unwrap(request(base, f"/kilo/path?{query}"))
    require(isinstance(path, dict) and isinstance(path.get("directory"), str), f"path mismatch: {path!r}")

    agents = unwrap(request(base, f"/kilo/agent?{query}"))
    require(isinstance(agents, list) and agents, "agent list must be non-empty")
    visible = [a for a in agents if isinstance(a, dict) and not a.get("hidden") and a.get("mode") != "subagent"]
    names = [str(a.get("name") or a.get("id") or "") for a in visible]
    require("code" in names, f"official Kilo code agent missing; visible agents={names!r}")
    require("build" not in names, f"unpatched build agent leaked through product API: {names!r}")

    providers = unwrap(request(base, f"/kilo/provider?{query}"))
    require(isinstance(providers, dict), "/provider must return an object")
    require(isinstance(providers.get("all"), list), "/provider.all must be an array")
    require(isinstance(providers.get("connected"), list), "/provider.connected must be an array")
    require(isinstance(providers.get("default"), dict), "/provider.default must be an object")

    created = unwrap(request(
        base,
        f"/kilo/session?{query}",
        method="POST",
        payload={"agent": "code"},
    ))
    require(isinstance(created, dict) and isinstance(created.get("id"), str), f"session.create mismatch: {created!r}")
    sid = created["id"]
    sidq = urllib.parse.quote(sid, safe="")

    sessions = unwrap(request(base, f"/kilo/session?{directory_query(project, {'limit': 50})}"))
    require(isinstance(sessions, list), "session.list must be an array")
    require(any(isinstance(s, dict) and s.get("id") == sid for s in sessions), "created session missing from list")

    messages = unwrap(request(base, f"/kilo/session/{sidq}/message?{directory_query(project, {'limit': 10})}"))
    require(isinstance(messages, list), "session messages must be an array")
    for item in messages:
        require(isinstance(item, dict) and isinstance(item.get("info"), dict) and isinstance(item.get("parts"), list),
                f"production message must be {{info, parts}}: {item!r}")

    statuses = unwrap(request(base, f"/kilo/session/status?{query}"))
    require(isinstance(statuses, dict), "session/status must be an object")

    permissions = unwrap(request(base, f"/kilo/permission?{query}"))
    require(isinstance(permissions, list), "permission list must be an array")
    questions = unwrap(request(base, f"/kilo/question?{query}"))
    require(isinstance(questions, list), "question list must be an array")

    event = first_global_event(base, project)
    require(isinstance(event, dict), "global event must be an object")
    payload = event.get("payload", event)
    require(isinstance(payload, dict) and isinstance(payload.get("type"), str), f"global event payload mismatch: {event!r}")

    print(json.dumps({
        "ok": True,
        "project": project,
        "agents": names,
        "providers": len(providers["all"]),
        "session": sid,
        "event": payload.get("type"),
    }, indent=2))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ContractError as error:
        print(f"PRODUCT CONTRACT FAILURE: {error}", file=sys.stderr)
        raise SystemExit(1)
