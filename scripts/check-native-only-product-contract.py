#!/usr/bin/env python3
import json
import sys
import urllib.error
import urllib.parse
import urllib.request


def request(base, path, method="GET", payload=None, expected=200):
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            body = response.read()
            status = response.status
    except urllib.error.HTTPError as error:
        body = error.read()
        status = error.code
    if status != expected:
        raise AssertionError(f"{method} {path}: expected {expected}, got {status}: {body[:1000]!r}")
    if not body:
        return None
    content_type = ""
    try:
        content_type = response.headers.get("content-type", "")
    except Exception:
        pass
    try:
        return json.loads(body.decode("utf-8"))
    except Exception:
        return body.decode("utf-8", errors="replace")


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def main():
    if len(sys.argv) != 3:
        print("usage: check-native-only-product-contract.py <base-url> <project>", file=sys.stderr)
        return 2
    base = sys.argv[1].rstrip("/")
    project = sys.argv[2]
    query = urllib.parse.urlencode({"directory": project})

    health = request(base, "/local/health")
    require(health.get("healthy") is True, f"native health mismatch: {health!r}")
    require(health.get("mode") == "native-only", f"native mode mismatch: {health!r}")
    require(health.get("compatibilityAvailable") is False, f"compat runtime unexpectedly available: {health!r}")

    status = request(base, "/local/status")
    runtime = status.get("runtime") if isinstance(status, dict) else None
    require(isinstance(runtime, dict) and runtime.get("mode") == "native-only", f"local status mismatch: {status!r}")

    path = request(base, "/local/path")
    require(path.get("directory") == project, f"local path mismatch: {path!r}")

    agents = request(base, "/local/agents")
    require(isinstance(agents, list) and any(a.get("id") == "code" for a in agents if isinstance(a, dict)),
            f"native agent catalog mismatch: {agents!r}")

    request(base, "/runtime/global/health", expected=503)

    accounts = request(base, f"/local/provider-accounts?{query}")
    require(isinstance(accounts, list), f"provider account list mismatch: {accounts!r}")
    kilo_account = next((item for item in accounts if isinstance(item, dict) and item.get("id") == "kilo"), None)
    require(isinstance(kilo_account, dict), f"Kilo provider account adapter missing: {accounts!r}")
    require(kilo_account.get("available") is False and kilo_account.get("connected") is False,
            f"native-only Kilo account must be unavailable: {kilo_account!r}")
    require(kilo_account.get("authModes") == ["account"], f"Kilo account auth modes mismatch: {kilo_account!r}")

    hosted = request(base, f"/runtime/hosted/status?{query}")
    require(hosted.get("available") is False and hosted.get("authenticated") is False,
            f"hosted compatibility status mismatch: {hosted!r}")

    catalog = request(base, f"/runtime/providers/catalog?{query}")
    require(isinstance(catalog, dict), f"provider catalog mismatch: {catalog!r}")
    hosted_meta = catalog.get("hosted")
    require(isinstance(hosted_meta, dict) and hosted_meta.get("available") is False,
            f"catalog hosted metadata mismatch: {catalog!r}")
    providers = catalog.get("all")
    require(isinstance(providers, list), f"catalog providers mismatch: {catalog!r}")
    require(not any(p.get("id") == "kilo" for p in providers if isinstance(p, dict)),
            f"hosted Kilo leaked into native-only catalog: {providers!r}")

    permissions = request(base, "/local/permissions")
    require(permissions == [], f"native-only permissions mismatch: {permissions!r}")
    questions = request(base, "/local/questions")
    require(questions == [], f"native-only questions mismatch: {questions!r}")

    session = request(base, f"/local/sessions?{query}", method="POST", payload={"title": "Native CI"}, expected=201)
    session_id = session.get("id")
    require(isinstance(session_id, str) and session_id.startswith("tls_"), f"native session id mismatch: {session!r}")

    renamed = request(base, f"/local/sessions/{urllib.parse.quote(session_id)}?{query}",
                      method="PATCH", payload={"title": "Native CI renamed"})
    require(renamed.get("title") == "Native CI renamed", f"native rename mismatch: {renamed!r}")

    sessions = request(base, "/local/sessions?limit=20")
    require(any(s.get("id") == session_id for s in sessions if isinstance(s, dict)),
            f"native session missing from list: {sessions!r}")

    result = request(base, f"/local/sessions/{urllib.parse.quote(session_id)}?{query}",
                     method="DELETE")
    require(result.get("deleted") is True, f"native delete mismatch: {result!r}")

    print("TL Studio native-only product contract: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
