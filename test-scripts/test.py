#!/usr/bin/env python3
"""
OmniGate integration test.

Setup is idempotent — API keys and source IDs are cached in Valkey,
so re-runs reuse existing devices and configs.

Usage:
    ADMIN_DEFAULT_PASSWORD=secret python test.py          # run / reuse env
    ADMIN_DEFAULT_PASSWORD=secret python test.py --reset  # wipe cache, recreate

Dependencies:
    pip install requests redis
"""
import argparse
import json
import os
import sys
import time

import requests
from redis import Redis

# ─── Connection ───────────────────────────────────────────────────────────────
BASE_URL   = os.getenv("BASE_URL",   "http://localhost:8090")
VALKEY_URL = os.getenv("VALKEY_URL", "redis://localhost:6380")
ADMIN_PASS = os.getenv("ADMIN_DEFAULT_PASSWORD")

GATE_ID     = "gate-test"
TRIGGER_URL = "https://picsum.photos/400/300"  # returns a real JPEG

# Minimal valid JPEG (just enough for content-type detection)
FAKE_JPEG = (
    b"\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"
    b"\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n"
    b"\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d"
    b"\xff\xd9"
)

# ─── Valkey cache keys ────────────────────────────────────────────────────────
P = "omnigate:test:"
K = {
    "type_camera": P + "type:camera:id",
    "type_scale":  P + "type:scale:id",
    "dev1_sid":    P + "dev1:source_id",
    "dev1_key":    P + "dev1:api_key",
    "dev2_sid":    P + "dev2:source_id",
    "dev2_key":    P + "dev2:api_key",
    "dev3_sid":    P + "dev3:source_id",
    "dev3_key":    P + "dev3:api_key",
}

# ─── Output ───────────────────────────────────────────────────────────────────
GREEN  = "\033[32m"
YELLOW = "\033[33m"
RED    = "\033[31m"
RESET  = "\033[0m"
BOLD   = "\033[1m"

def ok(msg):   print(f"  {GREEN}[OK]{RESET}   {msg}")
def skip(msg): print(f"  {YELLOW}[SKIP]{RESET} {msg}")
def info(msg): print(f"         {msg}")
def step(msg): print(f"\n{BOLD}── {msg}{RESET}")


def fail(msg):
    print(f"  {RED}[FAIL]{RESET} {msg}", file=sys.stderr)
    sys.exit(1)


# ─── Auth ─────────────────────────────────────────────────────────────────────
def login() -> dict:
    r = requests.post(
        f"{BASE_URL}/api/auth/login",
        json={"username": "admin", "password": ADMIN_PASS},
        timeout=10,
    )
    if r.status_code != 200:
        fail(f"Login failed ({r.status_code}): {r.text}")
    return {"Authorization": f"Bearer {r.json()['session_id']}"}


# ─── Setup helpers ────────────────────────────────────────────────────────────
def get_or_create_event_type(rv, hdrs, code, name, fields, cache_key):
    cached = rv.get(cache_key)
    if cached:
        skip(f"Event type '{code}'  (cached id={cached})")
        return cached

    all_types = requests.get(f"{BASE_URL}/api/v1/types", headers=hdrs, timeout=10).json()
    existing = next((t for t in all_types if t["code"] == code), None)
    if existing:
        rv.set(cache_key, existing["id"])
        skip(f"Event type '{code}'  (exists id={existing['id']})")
        return existing["id"]

    r = requests.post(
        f"{BASE_URL}/api/v1/types", headers=hdrs,
        json={"code": code, "name": name, "fields": fields},
        timeout=10,
    )
    if r.status_code != 201:
        fail(f"Cannot create event type '{code}': {r.text}")
    type_id = r.json()["id"]
    rv.set(cache_key, type_id)
    ok(f"Created event type '{code}'  (id={type_id})")
    return type_id


def get_or_create_api_key(rv, hdrs, sid_key, api_key_cache, name, perms):
    sid = rv.get(sid_key)
    key = rv.get(api_key_cache)
    if sid and key:
        skip(f"API key for '{name}'  (cached source_id={sid})")
        return sid, key

    r = requests.post(
        f"{BASE_URL}/api/auth/admin/keys", headers=hdrs,
        json={"name": name, "gate_id": GATE_ID, "permission_ids": perms},
        timeout=10,
    )
    if r.status_code != 201:
        fail(f"Cannot create API key for '{name}': {r.text}")
    data = r.json()
    sid, key = str(data["id"]), data["api_key"]
    rv.set(sid_key, sid)
    rv.set(api_key_cache, key)
    ok(f"Created API key for '{name}'  (source_id={sid})")
    return sid, key


def get_or_create_device_config(hdrs, source_id, event_type_id, **kw):
    r = requests.get(
        f"{BASE_URL}/api/v1/configs/devices/{source_id}", headers=hdrs, timeout=10
    )
    if r.status_code == 200:
        skip(f"Device config  (source_id={source_id})")
        return

    body = {
        "source_id":     source_id,
        "event_type_id": event_type_id,
        "gate_id":       GATE_ID,
        **kw,
    }
    r = requests.post(
        f"{BASE_URL}/api/v1/configs/devices", headers=hdrs, json=body, timeout=10
    )
    if r.status_code not in (200, 201):
        fail(f"Cannot create device config (source_id={source_id}): {r.text}")
    ok(f"Created device config  (source_id={source_id})")


# ─── Environment setup ────────────────────────────────────────────────────────
def setup(rv, hdrs):
    """
    Devices:
      1  – Camera  – simple ingest, no trigger
      2  – Scale   – ingest triggers puller → assumes Device 3
      3  – Camera  – puller target (no direct ingest in tests)
    """
    step("Event Types")
    cam_type_id = get_or_create_event_type(
        rv, hdrs,
        code="camera_recognition",
        name="Camera Recognition",
        fields={
            "plate":      {"type": "string", "required": True},
            "confidence": {"type": "float",  "required": False},
            "direction":  {"type": "string", "required": False},
        },
        cache_key=K["type_camera"],
    )
    scale_type_id = get_or_create_event_type(
        rv, hdrs,
        code="scale_weight",
        name="Scale Weight",
        fields={"weight_kg": {"type": "float", "required": True}},
        cache_key=K["type_scale"],
    )

    step("API Keys")
    dev1_sid, dev1_key = get_or_create_api_key(
        rv, hdrs, K["dev1_sid"], K["dev1_key"],
        name="Test – Device 1 (Camera)",
        perms=["ingest:events"],
    )
    dev2_sid, dev2_key = get_or_create_api_key(
        rv, hdrs, K["dev2_sid"], K["dev2_key"],
        name="Test – Device 2 (Scale + trigger)",
        perms=["ingest:events"],
    )
    dev3_sid, dev3_key = get_or_create_api_key(
        rv, hdrs, K["dev3_sid"], K["dev3_key"],
        name="Test – Device 3 (Camera, puller target)",
        perms=["ingest:events"],
    )

    step("Device Configs")
    # Device 1 – camera, plain ingest
    get_or_create_device_config(
        hdrs, dev1_sid, cam_type_id,
        data_type="json",
        data_mapping={
            "plate":      "$.plate",
            "confidence": "$.confidence",
            "direction":  "$.direction",
        },
        trigger_enabled=False,
    )
    # Device 2 – scale, triggers puller to assume Device 3
    get_or_create_device_config(
        hdrs, dev2_sid, scale_type_id,
        data_type="json",
        data_mapping={"weight_kg": "$.weight_kg"},
        trigger_enabled=True,
        trigger_url=TRIGGER_URL,
        trigger_source_id=dev3_sid,
    )
    # Device 3 – camera, receives pulled data
    get_or_create_device_config(
        hdrs, dev3_sid, cam_type_id,
        data_type="json",
        data_mapping={
            "plate":      "$.plate",
            "confidence": "$.confidence",
        },
        trigger_enabled=False,
    )

    return dev1_sid, dev1_key, dev2_sid, dev2_key, dev3_sid


# ─── Core event helpers ───────────────────────────────────────────────────────
def count_events(hdrs, source_id) -> int:
    r = requests.get(f"{BASE_URL}/api/v1/events", headers=hdrs, timeout=10)
    if r.status_code != 200:
        return 0
    return sum(1 for e in r.json() if str(e.get("source_id")) == str(source_id))


def get_latest_event(hdrs, source_id):
    r = requests.get(f"{BASE_URL}/api/v1/events", headers=hdrs, timeout=10)
    if r.status_code != 200:
        return None
    events = [e for e in r.json() if str(e.get("source_id")) == str(source_id)]
    return events[-1] if events else None


def wait_for_new_event(hdrs, source_id, count_before, label, timeout=25) -> dict:
    info(f"Waiting up to {timeout}s for '{label}' event to appear in Core …")
    deadline = time.time() + timeout
    while time.time() < deadline:
        if count_events(hdrs, source_id) > count_before:
            ev = get_latest_event(hdrs, source_id)
            ok(f"'{label}' event in Core  (id={ev.get('id')}, data={ev.get('data')})")
            return ev
        time.sleep(1)
    fail(f"Timeout: '{label}' event not found in Core after {timeout}s")


# ─── Test 2.1 – Simple camera ingest (Device 1) ───────────────────────────────
def test_camera_ingest(hdrs, dev1_sid, dev1_key):
    step("Test 2.1  ·  Camera Ingest  (Device 1, no trigger)")

    before = count_events(hdrs, dev1_sid)

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        data={"payload": json.dumps({
            "event_type": "camera_recognition",
            "plate":      "AA1234BB",
            "confidence": 0.98,
            "direction":  "in",
        })},
        files={"image": ("plate.jpg", FAKE_JPEG, "image/jpeg")},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")

    resp_data = r.json()
    ok(f"Ingestor accepted  (transaction_id={resp_data.get('transaction_id')})")

    wait_for_new_event(hdrs, dev1_sid, before, "Device 1 camera")


# ─── Test 2.2 – Scale ingest + puller trigger (Device 2 → Device 3) ──────────
def test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid):
    step("Test 2.2  ·  Scale Ingest + Puller Trigger  (Device 2 → Device 3)")

    before_dev2 = count_events(hdrs, dev2_sid)
    before_dev3 = count_events(hdrs, dev3_sid)

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev2_key},
        data={"payload": json.dumps({
            "event_type": "scale_weight",
            "weight_kg":  25400.0,
        })},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")

    resp_data = r.json()
    ok(f"Ingestor accepted  (transaction_id={resp_data.get('transaction_id')})")

    # Adapter processes Device 2 event → publishes to events:puller
    wait_for_new_event(hdrs, dev2_sid, before_dev2, "Device 2 scale", timeout=20)

    # Puller picks up → fetches TRIGGER_URL → ingests as Device 3
    wait_for_new_event(hdrs, dev3_sid, before_dev3, "Device 3 camera (puller)", timeout=30)


# ─── Main ─────────────────────────────────────────────────────────────────────
def main():
    if not ADMIN_PASS:
        fail("ADMIN_DEFAULT_PASSWORD env var is not set")

    parser = argparse.ArgumentParser(description="OmniGate integration test")
    parser.add_argument(
        "--reset", action="store_true",
        help="Clear Valkey test cache and recreate all devices/configs",
    )
    args = parser.parse_args()

    try:
        rv = Redis.from_url(VALKEY_URL, decode_responses=True, socket_connect_timeout=3)
        rv.ping()
    except Exception as e:
        fail(f"Cannot connect to Valkey at {VALKEY_URL}: {e}")

    if args.reset:
        for k in K.values():
            rv.delete(k)
        print(f"  {YELLOW}[reset]{RESET} Valkey test cache cleared")

    print(f"\n{BOLD}{'=' * 52}{RESET}")
    print(f"{BOLD}  OmniGate Integration Test{RESET}")
    print(f"  Gateway : {BASE_URL}")
    print(f"  Valkey  : {VALKEY_URL}")
    print(f"{BOLD}{'=' * 52}{RESET}")

    step("Login")
    hdrs = login()
    ok("Logged in as admin")

    step("Environment Setup")
    dev1_sid, dev1_key, dev2_sid, dev2_key, dev3_sid = setup(rv, hdrs)

    test_camera_ingest(hdrs, dev1_sid, dev1_key)
    test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid)

    print(f"\n{BOLD}{'=' * 52}{RESET}")
    print(f"{BOLD}{GREEN}  All tests passed ✓{RESET}")
    print(f"{BOLD}{'=' * 52}{RESET}\n")


if __name__ == "__main__":
    main()
