#!/usr/bin/env python3
"""Exercise Kilo v7.6.2's production coding path against a local fake LLM."""

from __future__ import annotations

import json
import os
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

EXPECTED = "E2E_PRODUCT_OK"
FILE_CONTENT = "KILO_LOCAL_UI_OK"


class E2EError(RuntimeError):
    pass


def require(condition: bool, message: str):
    if not condition:
        raise E2EError(message)


def unwrap(value):
    if isinstance(value, dict) and "data" in value:
        return value["data"]
    return value


def request(base: str, path: str, method: str = "GET", payload=None, timeout=30):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base.rstrip("/") + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as res:
            raw = res.read()
            if not raw:
                return None
            ctype = res.headers.get("Content-Type", "")
            if "json" not in ctype:
                raise E2EError(f"{method} {path}: expected JSON, got {ctype!r}")
            return json.loads(raw)
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise E2EError(f"{method} {path}: HTTP {error.code}: {detail}") from error


def routed(path: str, project: str, **params) -> str:
    query = {"directory": project, **params}
    return f"{path}?{urllib.parse.urlencode(query)}"


def sse_events(base: str, project: str, sink: list[dict], ready: threading.Event, stop: threading.Event):
    try:
        req = urllib.request.Request(
            base.rstrip("/") + routed("/kilo/global/event", project),
            headers={"Accept": "text/event-stream", "Cache-Control": "no-cache"},
        )
        with urllib.request.urlopen(req, timeout=60) as res:
            data_lines: list[str] = []
            while not stop.is_set():
                raw = res.readline()
                if not raw:
                    break
                line = raw.decode("utf-8", errors="replace").rstrip("\r\n")
                if not line:
                    if not data_lines:
                        continue
                    try:
                        envelope = json.loads("\n".join(data_lines))
                        if isinstance(envelope, dict):
                            sink.append(envelope)
                            payload = envelope.get("payload", envelope)
                            if isinstance(payload, dict) and payload.get("type") == "server.connected":
                                ready.set()
                    finally:
                        data_lines = []
                    continue
                if line.startswith("data:"):
                    data_lines.append(line[5:].lstrip())
    except Exception as error:
        sink.append({"type": "test.sse.error", "message": str(error)})
        ready.set()


def assistant_text(envelope: dict) -> str:
    if not isinstance(envelope, dict):
        return ""
    info = envelope.get("info")
    if not isinstance(info, dict) or info.get("role") != "assistant":
        return ""
    parts = envelope.get("parts")
    if not isinstance(parts, list):
        return ""
    return "\n".join(
        part.get("text", "")
        for part in parts
        if isinstance(part, dict) and part.get("type") == "text" and isinstance(part.get("text"), str)
    )


def has_completed_write(envelope: dict) -> bool:
    parts = envelope.get("parts") if isinstance(envelope, dict) else None
    if not isinstance(parts, list):
        return False
    return any(
        isinstance(part, dict)
        and part.get("type") == "tool"
        and part.get("tool") == "write"
        and isinstance(part.get("state"), dict)
        and part["state"].get("status") == "completed"
        for part in parts
    )


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: check-kilo-prompt-e2e.py <launcher-base-url>", file=sys.stderr)
        return 2
    base = sys.argv[1].rstrip("/")

    local = request(base, "/local/status")
    project = local.get("project") if isinstance(local, dict) else None
    require(isinstance(project, str) and project, f"local project missing: {local!r}")

    agents = unwrap(request(base, routed("/kilo/agent", project)))
    require(isinstance(agents, list), f"agent response mismatch: {agents!r}")
    visible = [a for a in agents if isinstance(a, dict) and not a.get("hidden") and a.get("mode") != "subagent"]
    names = [str(a.get("name") or a.get("id") or "") for a in visible]
    require("code" in names, f"product code agent missing: {names!r}")
    require("build" not in names, f"raw build agent leaked through product API: {names!r}")

    provider_state = unwrap(request(base, routed("/kilo/provider", project)))
    require(isinstance(provider_state, dict), f"provider response mismatch: {provider_state!r}")
    providers = provider_state.get("all")
    require(isinstance(providers, list), "provider.all missing")
    test_provider = next((p for p in providers if isinstance(p, dict) and p.get("id") == "test"), None)
    require(test_provider is not None, f"test provider missing: {[p.get('id') for p in providers if isinstance(p, dict)]!r}")
    models = test_provider.get("models")
    if isinstance(models, dict):
        require("test-model" in models, f"test-model missing from provider: {models!r}")
    elif isinstance(models, list):
        require(any(isinstance(m, dict) and (m.get("id") == "test-model" or m.get("modelID") == "test-model") for m in models),
                f"test-model missing from provider: {models!r}")
    else:
        raise E2EError(f"test provider models have invalid shape: {models!r}")

    # Match the official product flow: create the Session independently, then
    # select the effective agent/model on prompt_async.
    created = unwrap(request(
        base,
        routed("/kilo/session", project),
        method="POST",
        payload={"agent": "code"},
    ))
    require(isinstance(created, dict) and isinstance(created.get("id"), str), f"session creation failed: {created!r}")
    session_id = created["id"]
    sid = urllib.parse.quote(session_id, safe="")

    events: list[dict] = []
    ready = threading.Event()
    stop = threading.Event()
    thread = threading.Thread(target=sse_events, args=(base, project, events, ready, stop), daemon=True)
    thread.start()
    require(ready.wait(5), f"global SSE did not connect: {events!r}")
    require(not any(event.get("type") == "test.sse.error" for event in events), f"SSE failed: {events!r}")

    request(
        base,
        routed(f"/kilo/session/{sid}/prompt_async", project),
        method="POST",
        payload={
            "agent": "code",
            "model": {"providerID": "test", "modelID": "test-model"},
            "parts": [{"type": "text", "text": "Create the requested fixture file, then confirm completion."}],
        },
    )

    deadline = time.time() + 45
    messages: list[dict] = []
    saw_running = False
    while time.time() < deadline:
        statuses = unwrap(request(base, routed("/kilo/session/status", project)))
        statuses = statuses if isinstance(statuses, dict) else {}
        status = statuses.get(session_id)
        if isinstance(status, dict) and status.get("type") != "idle":
            saw_running = True

        messages = unwrap(request(base, routed(f"/kilo/session/{sid}/message", project, limit=200)))
        messages = messages if isinstance(messages, list) else []
        if any(EXPECTED in assistant_text(message) for message in messages):
            break

        for message in messages:
            info = message.get("info") if isinstance(message, dict) else None
            if isinstance(info, dict) and info.get("role") == "assistant" and info.get("error"):
                raise E2EError(f"assistant failed before fixture reply: {info.get('error')!r}; messages={messages!r}")

        sse_error = next((event for event in events if event.get("type") == "test.sse.error"), None)
        if sse_error:
            raise E2EError(f"SSE failed while waiting for completion: {sse_error!r}")
        time.sleep(0.1)
    else:
        raise E2EError(f"timed out waiting for assistant reply; saw_running={saw_running!r} messages={messages!r} events={events!r}")

    stop.set()

    users = [m for m in messages if isinstance(m, dict) and isinstance(m.get("info"), dict) and m["info"].get("role") == "user"]
    assistants = [m for m in messages if isinstance(m, dict) and isinstance(m.get("info"), dict) and m["info"].get("role") == "assistant"]
    require(users, f"no projected user message: {messages!r}")
    require(assistants, f"no projected assistant message: {messages!r}")
    require(any(EXPECTED in assistant_text(m) for m in assistants), f"fixture reply missing: {assistants!r}")
    require(any(has_completed_write(m) for m in assistants), f"completed write tool part missing: {assistants!r}")

    target = os.path.join(project, "hello.txt")
    require(os.path.isfile(target), f"Kilo did not create {target}")
    with open(target, "r", encoding="utf-8") as handle:
        actual = handle.read()
    require(actual == FILE_CONTENT, f"file content mismatch: {actual!r}")

    interesting = []
    for envelope in events:
        payload = envelope.get("payload", envelope) if isinstance(envelope, dict) else {}
        if isinstance(payload, dict):
            props = payload.get("properties") if isinstance(payload.get("properties"), dict) else {}
            info = props.get("info") if isinstance(props.get("info"), dict) else {}
            part = props.get("part") if isinstance(props.get("part"), dict) else {}
            sid_from_event = props.get("sessionID") or info.get("sessionID") or part.get("sessionID")
            if sid_from_event == session_id:
                interesting.append(payload.get("type"))
    require(any(t in {"message.updated", "message.part.updated", "session.status", "session.idle"} for t in interesting),
            f"no production session/message event observed for session: {interesting!r}")

    print(json.dumps({
        "ok": True,
        "session": session_id,
        "agent": "code",
        "model": "test/test-model",
        "messages": len(messages),
        "events": interesting,
        "saw_running": saw_running,
        "file": target,
        "reply": EXPECTED,
    }, indent=2))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except E2EError as error:
        print(f"PRODUCT E2E FAILURE: {error}", file=sys.stderr)
        raise SystemExit(1)
