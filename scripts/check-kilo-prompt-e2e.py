#!/usr/bin/env python3
"""Exercise Kilo's real v2 agent loop against the local fake LLM fixture."""

from __future__ import annotations

import json
import sys
import threading
import time
import urllib.parse
import urllib.request

EXPECTED = "E2E_PROTOCOL_OK"


class E2EError(RuntimeError):
    pass


def require(condition: bool, message: str):
    if not condition:
        raise E2EError(message)


def request(base: str, path: str, method: str = "GET", payload=None, timeout=30):
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
        content_type = res.headers.get("Content-Type", "")
        if "json" not in content_type:
            raise E2EError(f"{method} {path}: expected JSON, got {content_type!r}")
        return json.loads(raw)


def sse_events(base: str, sink: list[dict], ready: threading.Event, stop: threading.Event):
    try:
        req = urllib.request.Request(
            base.rstrip("/") + "/kilo/api/event",
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
                        event = json.loads("\n".join(data_lines))
                        if isinstance(event, dict):
                            sink.append(event)
                            if event.get("type") == "server.connected":
                                ready.set()
                    finally:
                        data_lines = []
                    continue
                if line.startswith("data:"):
                    data_lines.append(line[5:].lstrip())
    except Exception as error:
        sink.append({"type": "test.sse.error", "data": {"message": str(error)}})
        ready.set()


def assistant_text(message: dict) -> str:
    content = message.get("content")
    if not isinstance(content, list):
        return ""
    return "\n".join(
        item.get("text", "")
        for item in content
        if isinstance(item, dict) and item.get("type") == "text" and isinstance(item.get("text"), str)
    )


def wait_for_test_model(base: str, timeout: float = 25.0):
    """Wait for Location plugins/config to finish populating the v2 catalog."""
    deadline = time.time() + timeout
    last_models: list[dict] = []
    while time.time() < deadline:
        payload = request(base, "/kilo/api/model")
        last_models = payload.get("data", []) if isinstance(payload, dict) else []
        match = next(
            (
                item
                for item in last_models
                if item.get("providerID") == "test" and item.get("id") == "test-model"
            ),
            None,
        )
        if match is not None:
            return match
        time.sleep(0.2)

    summary = [
        f"{item.get('providerID')}/{item.get('id')}"
        for item in last_models
        if isinstance(item, dict)
    ]
    raise E2EError(
        "test/test-model was not exposed by /api/model after waiting for catalog boot; "
        f"last models={summary[:30]!r}"
    )


def event_for_session(events: list[dict], session_id: str, event_type: str):
    return next(
        (
            event
            for event in events
            if event.get("type") == event_type
            and isinstance(event.get("data"), dict)
            and event["data"].get("sessionID") == session_id
        ),
        None,
    )


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: check-kilo-prompt-e2e.py <launcher-base-url>", file=sys.stderr)
        return 2
    base = sys.argv[1].rstrip("/")

    # Kilo's Location plugins populate the catalog asynchronously. Health only
    # proves that the HTTP server is ready, not that the configured provider has
    # completed catalog registration.
    test_model = wait_for_test_model(base)
    require(test_model is not None, "test/test-model was not exposed by /api/model")

    agents = request(base, "/kilo/api/agent").get("data", [])
    visible = [item for item in agents if not item.get("hidden") and item.get("mode") != "subagent"]
    agent = next((item for item in visible if item.get("id") == "code"), visible[0] if visible else None)
    require(agent is not None and isinstance(agent.get("id"), str), "no usable Kilo agent found")

    created = request(
        base,
        "/kilo/api/session",
        method="POST",
        payload={
            "agent": agent["id"],
            "model": {"providerID": "test", "id": "test-model"},
        },
    )
    session = created.get("data", {})
    session_id = session.get("id")
    require(isinstance(session_id, str) and session_id, "session creation did not return an ID")
    sid = urllib.parse.quote(session_id, safe="")

    events: list[dict] = []
    ready = threading.Event()
    stop = threading.Event()
    thread = threading.Thread(target=sse_events, args=(base, events, ready, stop), daemon=True)
    thread.start()
    require(ready.wait(5), "global SSE did not connect before prompt")
    require(not any(event.get("type") == "test.sse.error" for event in events), f"SSE failed: {events!r}")

    admitted = request(
        base,
        f"/kilo/api/session/{sid}/prompt",
        method="POST",
        payload={
            "prompt": {"text": "Reply with the CI fixture response."},
            "delivery": "queue",
        },
    )
    receipt = admitted.get("data", {})
    require(receipt.get("sessionID") == session_id, "prompt admission returned the wrong session")
    require(isinstance(receipt.get("id"), str), "prompt admission did not return a message ID")

    # v7.6.2 intentionally exposes /session/:id/wait as an unavailable operation.
    # Synchronize using the public v2 live stream plus projected messages instead:
    # prompt() wakes SessionExecution asynchronously, step.failed/step.ended report
    # settlement, and /message remains the reconnect-safe source of truth.
    deadline = time.time() + 45
    messages: list[dict] = []
    saw_active = False
    last_active: dict = {}
    while time.time() < deadline:
        active_payload = request(base, "/kilo/api/session/active")
        last_active = active_payload.get("data", {}) if isinstance(active_payload, dict) else {}
        if session_id in last_active:
            saw_active = True

        page = request(base, f"/kilo/api/session/{sid}/message?order=asc&limit=200")
        messages = page.get("data", []) if isinstance(page, dict) else []
        if any(message.get("type") == "assistant" and EXPECTED in assistant_text(message) for message in messages):
            break

        failed = event_for_session(events, session_id, "session.next.step.failed")
        if failed is not None:
            raise E2EError(f"Kilo agent step failed before fixture reply: {failed!r}; messages={messages!r}")

        sse_error = next((event for event in events if event.get("type") == "test.sse.error"), None)
        if sse_error is not None:
            raise E2EError(f"SSE failed while waiting for agent completion: {sse_error!r}")

        time.sleep(0.1)
    else:
        raise E2EError(
            "timed out waiting for projected assistant reply; "
            f"saw_active={saw_active!r} active={last_active!r} messages={messages!r} events={events!r}"
        )

    stop.set()

    users = [message for message in messages if message.get("type") == "user"]
    assistants = [message for message in messages if message.get("type") == "assistant"]
    require(users, f"no projected user message found: {messages!r}")
    require(assistants, f"no projected assistant message found: {messages!r}")
    require(any(EXPECTED in assistant_text(message) for message in assistants), f"fixture reply missing: {assistants!r}")

    for message in messages:
        require("info" not in message and "parts" not in message, "public v2 messages unexpectedly exposed info/parts")

    matching = next(message for message in assistants if EXPECTED in assistant_text(message))
    model = matching.get("model")
    require(isinstance(model, dict), "assistant.model must be a Model.Ref")
    require(model.get("providerID") == "test" and model.get("id") == "test-model", f"assistant model mismatch: {model!r}")
    require(isinstance(matching.get("content"), list), "assistant.content must be an array")

    # Verify the same provider response was observable as live v2 stream deltas,
    # not only after projection/reload.
    event_deadline = time.time() + 5
    while time.time() < event_deadline:
        streamed = "".join(
            str(event.get("data", {}).get("delta", ""))
            for event in events
            if event.get("type") == "session.next.text.delta"
            and event.get("data", {}).get("sessionID") == session_id
        )
        if EXPECTED in streamed:
            break
        time.sleep(0.05)
    else:
        raise E2EError(f"live session.next.text.delta did not contain {EXPECTED!r}; events={events!r}")

    require(
        event_for_session(events, session_id, "session.next.step.ended") is not None,
        "live SSE did not publish session.next.step.ended",
    )

    print(
        json.dumps(
            {
                "ok": True,
                "session": session_id,
                "agent": agent["id"],
                "model": "test/test-model",
                "messages": len(messages),
                "live_events": len(events),
                "saw_active": saw_active,
                "reply": EXPECTED,
            },
            indent=2,
        )
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except E2EError as error:
        print(f"E2E FAILURE: {error}", file=sys.stderr)
        raise SystemExit(1)
