#!/usr/bin/env python3
"""
OmniGate integration test.

Setup is idempotent — API keys and source IDs are cached in Valkey,
so re-runs reuse existing devices and configs.

Device layout (multi-gate):
  GATE_1 (gate-north):
    Device 1 – Camera (JSON)      – multipart ingest with image, no trigger
    Device 2 – Scale  (JSON)      – ingest triggers puller → targets Device 3
    Device 3 – Camera (JSON)      – puller pull target; owns trigger_url in its config
  GATE_2 (gate-south):
    Device 4 – Scale  (JSON flat) – raw JSON body & auto-collected form fields
    Device 5 – Scale  (XML)       – raw XML body

Puller flow:
  Adapter processes Device 2 event
    → publishes {trigger_source_id: dev3_sid, gate_id: GATE_1, ...} to events:puller stream
  Puller reads trigger_source_id + gate_id
    → GET /api/v1/configs/devices/{trigger_source_id} on Core
    → reads trigger_url from Device 3 config
    → fetches trigger_url
    → POSTs result to Ingestor as Device 3, including gate_id in envelope

Tests:
  2.1  Camera ingest (multipart + image)              — raw_payload + gate_id verified
  2.2  Puller config resolution                       — config structure + gate_id verified
  2.3  Scale ingest + automatic puller trigger        — raw_payload + gate_id verified
  2.4  Manual trigger via Core API                    — raw_payload + gate_id verified
  2.5  Raw JSON body (application/json)               — raw_payload + gate_id verified
  2.6  Auto-collected form fields (no payload field)  — raw_payload + gate_id verified
  2.7  Raw XML body (application/xml)                 — raw_payload + gate_id verified
  2.8  Transaction isolation (GATE_1 ≠ GATE_2)        — transaction_id isolation

Usage:
    ADMIN_DEFAULT_PASSWORD=secret python test.py          # run / reuse env
    ADMIN_DEFAULT_PASSWORD=secret python test.py --reset  # wipe cache, recreate
    NOTE: --reset required when upgrading from the old single-gate (gate-test) setup.

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

GATE_1      = "gate-north"   # Dev1, Dev2, Dev3
GATE_2      = "gate-south"   # Dev4, Dev5
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
    "type_camera":     P + "type:camera:id",
    "type_scale":      P + "type:scale:id",
    "type_flat_scale": P + "type:flat_scale:id",
    "type_xml_scale":  P + "type:xml_scale:id",
    "dev1_sid":        P + "dev1:source_id",
    "dev1_key":        P + "dev1:api_key",
    "dev2_sid":        P + "dev2:source_id",
    "dev2_key":        P + "dev2:api_key",
    "dev3_sid":        P + "dev3:source_id",
    "dev3_key":        P + "dev3:api_key",
    "dev4_sid":        P + "dev4:source_id",
    "dev4_key":        P + "dev4:api_key",
    "dev5_sid":        P + "dev5:source_id",
    "dev5_key":        P + "dev5:api_key",
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
def warn(msg): print(f"  {YELLOW}[WARN]{RESET} {msg}")


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


def get_or_create_api_key(rv, hdrs, sid_key, api_key_cache, name, gate_id, perms):
    sid = rv.get(sid_key)
    key = rv.get(api_key_cache)
    if sid and key:
        skip(f"API key for '{name}'  (cached source_id={sid})")
        return sid, key

    r = requests.post(
        f"{BASE_URL}/api/auth/admin/keys", headers=hdrs,
        json={"name": name, "gate_id": gate_id, "permission_ids": perms},
        timeout=10,
    )
    if r.status_code != 201:
        fail(f"Cannot create API key for '{name}': {r.text}")
    data = r.json()
    sid, key = str(data["id"]), data["api_key"]
    rv.set(sid_key, sid)
    rv.set(api_key_cache, key)
    ok(f"Created API key for '{name}'  (source_id={sid}, gate={gate_id})")
    return sid, key


def ensure_device_config(hdrs, source_id, event_type_id, gate_id, **kw):
    """Create or update a device config to match the expected settings."""
    body = {
        "source_id":     source_id,
        "event_type_id": event_type_id,
        "gate_id":       gate_id,
        **kw,
    }
    r = requests.get(
        f"{BASE_URL}/api/v1/configs/devices/{source_id}", headers=hdrs, timeout=10
    )
    if r.status_code == 200:
        cfg_id = r.json().get("id")
        ur = requests.put(
            f"{BASE_URL}/api/v1/configs/devices/{cfg_id}", headers=hdrs, json=body, timeout=10
        )
        if ur.status_code not in (200, 204):
            fail(f"Cannot update device config (source_id={source_id}): {ur.text}")
        skip(f"Device config ensured  (source_id={source_id}, gate={gate_id}, id={cfg_id})")
        return cfg_id

    cr = requests.post(
        f"{BASE_URL}/api/v1/configs/devices", headers=hdrs, json=body, timeout=10
    )
    if cr.status_code not in (200, 201):
        fail(f"Cannot create device config (source_id={source_id}): {cr.text}")
    cfg_id = cr.json().get("id")
    ok(f"Created device config  (source_id={source_id}, gate={gate_id}, id={cfg_id})")
    return cfg_id


# ─── Environment setup ────────────────────────────────────────────────────────
def setup(rv, hdrs):
    """
    Devices on GATE_1 (gate-north):
      1  – Camera  – multipart ingest with image, no trigger
      2  – Scale   – ingest triggers puller → targets Device 3 via trigger_source_id
      3  – Camera  – puller pull target; owns trigger_url in its own config

    Devices on GATE_2 (gate-south):
      4  – Scale (flat JSON) – raw JSON body & auto-collected form-fields tests
      5  – Scale (XML)       – raw XML body test
    """
    step("Event Types")
    cam_type_id = get_or_create_event_type(
        rv, hdrs,
        code="camera_recognition",
        name="Camera Recognition",
        fields={
            "plate":        {"type": "string", "required": True},
            "confidence":   {"type": "float",  "required": False},
            "direction":    {"type": "string", "required": False},
            "region":       {"type": "string", "required": False},
            "vehicle_type": {"type": "string", "required": False},
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
    flat_scale_type_id = get_or_create_event_type(
        rv, hdrs,
        code="scale_weight_flat",
        name="Scale Weight (flat)",
        fields={"weight_kg": {"type": "float", "required": True}},
        cache_key=K["type_flat_scale"],
    )
    xml_scale_type_id = get_or_create_event_type(
        rv, hdrs,
        code="scale_weight_xml",
        name="Scale Weight (XML)",
        fields={"weight_kg": {"type": "float", "required": True}},
        cache_key=K["type_xml_scale"],
    )

    step("API Keys")
    dev1_sid, dev1_key = get_or_create_api_key(
        rv, hdrs, K["dev1_sid"], K["dev1_key"],
        name="Test – Device 1 (Camera)",
        gate_id=GATE_1,
        perms=["ingest:events"],
    )
    dev2_sid, dev2_key = get_or_create_api_key(
        rv, hdrs, K["dev2_sid"], K["dev2_key"],
        name="Test – Device 2 (Scale + trigger)",
        gate_id=GATE_1,
        perms=["ingest:events"],
    )
    dev3_sid, _ = get_or_create_api_key(
        rv, hdrs, K["dev3_sid"], K["dev3_key"],
        name="Test – Device 3 (Camera, puller target)",
        gate_id=GATE_1,
        perms=["ingest:events"],
    )
    dev4_sid, dev4_key = get_or_create_api_key(
        rv, hdrs, K["dev4_sid"], K["dev4_key"],
        name="Test – Device 4 (Scale, flat JSON)",
        gate_id=GATE_2,
        perms=["ingest:events"],
    )
    dev5_sid, dev5_key = get_or_create_api_key(
        rv, hdrs, K["dev5_sid"], K["dev5_key"],
        name="Test – Device 5 (Scale, XML)",
        gate_id=GATE_2,
        perms=["ingest:events"],
    )

    step("Device Configs")
    ensure_device_config(
        hdrs, dev1_sid, cam_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={
            "plate":        "$.Event.Data.Content.VideoResult.plate.text",
            "confidence":   "$.Event.Data.Content.VideoResult.confidence",
            "direction":    "$.Event.Metadata.Traffic.dir",
            "region":       "$.Event.Data.Content.VideoResult.plate.region_code",
            "vehicle_type": "$.Event.Data.Content.VideoResult.vehicle.type",
        },
        trigger_enabled=False,
    )
    dev2_cfg_id = ensure_device_config(
        hdrs, dev2_sid, scale_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={"weight_kg": "$.Payload.Measurements.Weight.Value"},
        trigger_enabled=True,
        trigger_source_id=dev3_sid,
    )
    ensure_device_config(
        hdrs, dev3_sid, cam_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={
            "plate":      "$.Event.Data.Content.VideoResult.plate.text",
            "confidence": "$.Event.Data.Content.VideoResult.confidence",
        },
        trigger_enabled=False,
        trigger_url=TRIGGER_URL,
    )
    ensure_device_config(
        hdrs, dev4_sid, flat_scale_type_id,
        gate_id=GATE_2,
        data_type="json",
        data_mapping={"weight_kg": "$.weight_kg"},
        trigger_enabled=False,
    )
    ensure_device_config(
        hdrs, dev5_sid, xml_scale_type_id,
        gate_id=GATE_2,
        data_type="xml",
        data_mapping={"weight_kg": "$.scale.weight_kg"},
        trigger_enabled=False,
    )

    return (
        dev1_sid, dev1_key,
        dev2_sid, dev2_key, dev2_cfg_id,
        dev3_sid,
        dev4_sid, dev4_key,
        dev5_sid, dev5_key,
    )


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


# ─── Assertion helpers ────────────────────────────────────────────────────────
def assert_raw_payload(ev, label, expected_fragment=None):
    """Verify the event has a non-empty raw_payload, optionally checking a substring."""
    raw = ev.get("raw_payload") or ""
    if not raw:
        fail(f"'{label}': raw_payload is missing or empty  (event id={ev.get('id')})")
    if expected_fragment and expected_fragment not in raw:
        fail(
            f"'{label}': raw_payload does not contain expected '{expected_fragment}'\n"
            f"         raw_payload snippet: {raw[:300]}"
        )
    detail = f", contains {repr(expected_fragment)}" if expected_fragment else ""
    ok(f"raw_payload present  ({len(raw)} chars{detail})")


def assert_gate_id(ev, expected_gate, label):
    """Verify the event's gate_id matches the expected gate."""
    actual = ev.get("gate_id")
    if actual != expected_gate:
        fail(f"'{label}': gate_id mismatch — expected '{expected_gate}', got '{actual}'")
    ok(f"gate_id correct  (gate_id={actual})")


# ─── Test 2.1 – Simple camera ingest (Device 1, multipart + image) ────────────
def test_camera_ingest(hdrs, dev1_sid, dev1_key):
    step("Test 2.1  ·  Camera Ingest  (Device 1 — multipart/form-data + image)")

    before = count_events(hdrs, dev1_sid)

    raw_payload_str = json.dumps({
        "Event": {
            "Metadata": {
                "Source": "CAM-NORTH-01",
                "Session": "abc-123-xyz",
                "Traffic": {"dir": "in", "lane": 2},
            },
            "Data": {
                "Content": {
                    "VideoResult": {
                        "plate": {
                            "text": "AA1234BB",
                            "region_code": "UA",
                            "char_rects": [[1, 2], [3, 4]],
                        },
                        "confidence": 0.992,
                        "vehicle": {"type": "truck", "color": "white"},
                    }
                },
                "Diagnostics": {"temp": 45.2, "voltage": 12.1},
            },
        }
    })

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        data={"payload": raw_payload_str},
        files={"image": ("plate.jpg", FAKE_JPEG, "image/jpeg")},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")

    ok(f"Ingestor accepted  (transaction_id={r.json().get('transaction_id')})")
    ev = wait_for_new_event(hdrs, dev1_sid, before, "Device 1 camera")
    assert_raw_payload(ev, "Device 1 camera", expected_fragment="AA1234BB")
    assert_gate_id(ev, GATE_1, "Device 1 camera")


# ─── Test 2.2 – Puller config resolution ─────────────────────────────────────
def test_puller_config_resolution(hdrs, dev3_sid):
    step("Test 2.2  ·  Puller Config Resolution  (Device 3 owns trigger_url)")

    r = requests.get(
        f"{BASE_URL}/api/v1/configs/devices/{dev3_sid}", headers=hdrs, timeout=10
    )
    if r.status_code != 200:
        fail(f"Cannot fetch Device 3 config ({r.status_code}): {r.text}")

    cfg = r.json()
    trigger_url = cfg.get("trigger_url")
    if not trigger_url:
        fail(f"Device 3 config is missing trigger_url  (got: {cfg})")
    if cfg.get("gate_id") != GATE_1:
        fail(f"Device 3 config has wrong gate_id: expected '{GATE_1}', got '{cfg.get('gate_id')}'")

    ok(f"Device 3 config: trigger_url='{trigger_url}', gate_id='{cfg.get('gate_id')}'")
    info("Puller resolves trigger_url at runtime via GET /api/v1/configs/devices/{trigger_source_id}")


# ─── Test 2.3 – Scale ingest + automatic puller trigger ───────────────────────
def test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid):
    step("Test 2.3  ·  Scale Ingest + Automatic Puller Trigger  (Device 2 → Device 3)")

    before_dev2 = count_events(hdrs, dev2_sid)
    before_dev3 = count_events(hdrs, dev3_sid)

    scale_payload = {
        "Header": {"Version": "2.1", "Device": "SCALE-04"},
        "Payload": {
            "Measurements": {
                "Weight": {"Value": 25400.0, "Unit": "kg"},
                "Stability": True,
            },
            "Raw": [25399, 25401, 25400],
        },
    }

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev2_key},
        data={"payload": json.dumps(scale_payload)},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")

    ok(f"Ingestor accepted  (transaction_id={r.json().get('transaction_id')})")

    ev2 = wait_for_new_event(hdrs, dev2_sid, before_dev2, "Device 2 scale", timeout=20)
    assert_raw_payload(ev2, "Device 2 scale", expected_fragment="25400.0")
    assert_gate_id(ev2, GATE_1, "Device 2 scale")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before_dev3, "Device 3 (puller auto)", timeout=35)
    assert_raw_payload(ev3, "Device 3 (puller auto)")
    # gate_id must be GATE_1, not "system" (the Puller's own API-key gate).
    assert_gate_id(ev3, GATE_1, "Device 3 (puller auto)")


# ─── Test 2.4 – Manual trigger via Core API ───────────────────────────────────
def test_manual_trigger(hdrs, dev2_cfg_id, dev3_sid):
    step("Test 2.4  ·  Manual Trigger  (POST /configs/devices/{id}/trigger)")

    before_dev3 = count_events(hdrs, dev3_sid)

    r = requests.post(
        f"{BASE_URL}/api/v1/configs/devices/{dev2_cfg_id}/trigger",
        headers=hdrs,
        timeout=10,
    )
    if r.status_code not in (200, 202):
        fail(f"Manual trigger failed ({r.status_code}): {r.text}")

    ok(f"Manual trigger accepted  (response={r.json()})")
    info("Core published trigger_source_id + gate_id to events:puller stream")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before_dev3, "Device 3 (manual trigger)", timeout=35)
    assert_raw_payload(ev3, "Device 3 (manual trigger)")
    assert_gate_id(ev3, GATE_1, "Device 3 (manual trigger)")


# ─── Test 2.5 – Raw JSON body (no multipart wrapper) ─────────────────────────
def test_raw_json_body(hdrs, dev1_sid, dev1_key):
    step("Test 2.5  ·  Raw JSON Body  (Device 1 — application/json, no form wrapper)")

    before = count_events(hdrs, dev1_sid)

    body = {
        "Event": {
            "Metadata": {
                "Source": "CAM-NORTH-02",
                "Traffic": {"dir": "out", "lane": 1},
            },
            "Data": {
                "Content": {
                    "VideoResult": {
                        "plate": {"text": "BB5678CC", "region_code": "UA"},
                        "confidence": 0.981,
                        "vehicle": {"type": "car"},
                    }
                }
            },
        }
    }

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        json=body,
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected JSON body ({r.status_code}): {r.text}")

    ok(f"Ingestor accepted JSON body  (event_id={r.json().get('event_id')})")
    ev = wait_for_new_event(hdrs, dev1_sid, before, "Device 1 raw JSON body")
    assert_raw_payload(ev, "Device 1 raw JSON body", expected_fragment="BB5678CC")
    assert_gate_id(ev, GATE_1, "Device 1 raw JSON body")


# ─── Test 2.6 – Auto-collected form fields (no explicit payload field) ────────
def test_auto_form_fields(hdrs, dev4_sid, dev4_key):
    step("Test 2.6  ·  Auto-collected Form Fields  (Device 4 — multipart, no payload field)")

    before = count_events(hdrs, dev4_sid)

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev4_key},
        data={
            "weight_kg": "25400.0",
            "unit":      "kg",
            "stable":    "true",
        },
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected form-fields event ({r.status_code}): {r.text}")

    ok(f"Ingestor accepted form fields  (event_id={r.json().get('event_id')})")
    ev = wait_for_new_event(hdrs, dev4_sid, before, "Device 4 auto form fields")
    # Ingestor serialises collected fields to JSON: {"weight_kg": "25400.0", ...}
    assert_raw_payload(ev, "Device 4 auto form fields", expected_fragment="weight_kg")
    assert_gate_id(ev, GATE_2, "Device 4 auto form fields")


# ─── Test 2.7 – Raw XML body ──────────────────────────────────────────────────
def test_raw_xml_body(hdrs, dev5_sid, dev5_key):
    step("Test 2.7  ·  Raw XML Body  (Device 5 — application/xml)")

    before = count_events(hdrs, dev5_sid)

    xml_body = (
        "<?xml version='1.0' encoding='UTF-8'?>"
        "<scale><weight_kg>25400.0</weight_kg><unit>kg</unit></scale>"
    )

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev5_key, "Content-Type": "application/xml"},
        data=xml_body.encode("utf-8"),
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected XML body ({r.status_code}): {r.text}")

    ok(f"Ingestor accepted XML body  (event_id={r.json().get('event_id')})")
    ev = wait_for_new_event(hdrs, dev5_sid, before, "Device 5 raw XML body")
    assert_raw_payload(ev, "Device 5 raw XML body", expected_fragment="25400.0")
    assert_gate_id(ev, GATE_2, "Device 5 raw XML body")


# ─── Test 2.8 – Transaction isolation between gates ──────────────────────────
def test_transaction_isolation(hdrs, dev1_sid, dev4_sid):
    step("Test 2.8  ·  Transaction Isolation  (GATE_1 vs GATE_2)")

    ev_g1 = get_latest_event(hdrs, dev1_sid)
    ev_g2 = get_latest_event(hdrs, dev4_sid)

    if not ev_g1:
        fail("No GATE_1 events found — run tests 2.1–2.5 first")
    if not ev_g2:
        fail("No GATE_2 events found — run tests 2.6–2.7 first")

    tx1 = ev_g1.get("transaction_id")
    tx2 = ev_g2.get("transaction_id")

    if not tx1:
        fail(f"GATE_1 event (id={ev_g1.get('id')}) has no transaction_id")
    if not tx2:
        fail(f"GATE_2 event (id={ev_g2.get('id')}) has no transaction_id")

    if tx1 == tx2:
        fail(
            f"Transaction isolation violated: GATE_1 and GATE_2 share transaction_id={tx1}\n"
            f"         GATE_1 event id={ev_g1.get('id')}, GATE_2 event id={ev_g2.get('id')}"
        )

    ok(f"Transactions are isolated")
    info(f"GATE_1 ({GATE_1}) tx={tx1[:8]}…")
    info(f"GATE_2 ({GATE_2}) tx={tx2[:8]}…")


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

    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}  OmniGate Integration Test{RESET}")
    print(f"  Gateway : {BASE_URL}")
    print(f"  Valkey  : {VALKEY_URL}")
    print(f"  Gates   : {GATE_1} (Dev1–3)  ·  {GATE_2} (Dev4–5)")
    print(f"{BOLD}{'=' * 60}{RESET}")

    step("Login")
    hdrs = login()
    ok("Logged in as admin")

    step("Environment Setup")
    (
        dev1_sid, dev1_key,
        dev2_sid, dev2_key, dev2_cfg_id,
        dev3_sid,
        dev4_sid, dev4_key,
        dev5_sid, dev5_key,
    ) = setup(rv, hdrs)

    # ── GATE_1 tests ───────────────────────────────────────────────────────
    test_camera_ingest(hdrs, dev1_sid, dev1_key)
    test_puller_config_resolution(hdrs, dev3_sid)
    test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid)
    test_manual_trigger(hdrs, dev2_cfg_id, dev3_sid)
    test_raw_json_body(hdrs, dev1_sid, dev1_key)

    # ── GATE_2 tests ───────────────────────────────────────────────────────
    test_auto_form_fields(hdrs, dev4_sid, dev4_key)
    test_raw_xml_body(hdrs, dev5_sid, dev5_key)

    # ── Cross-gate regression ──────────────────────────────────────────────
    test_transaction_isolation(hdrs, dev1_sid, dev4_sid)

    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}{GREEN}  All tests passed ✓{RESET}")
    print(f"{BOLD}{'=' * 60}{RESET}\n")


if __name__ == "__main__":
    main()
