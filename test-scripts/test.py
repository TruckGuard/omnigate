#!/usr/bin/env python3
"""
OmniGate integration test.

Setup is idempotent — API keys and source IDs are cached in Valkey,
so re-runs reuse existing devices and configs.

Device layout (multi-gate):
  GATE_1 (gate-north):
    Device 1 – Camera (JSON)      – multipart ingest with image, no trigger
    Device 2 – Scale  (JSON)      – ingest triggers puller → targets Device 3
                                    triggers: [{source_id: dev3_sid}]
    Device 3 – Camera (JSON)      – puller pull target; trigger_url lives on THIS device's config
  GATE_2 (gate-south):
    Device 4 – Scale  (JSON flat) – raw JSON body & auto-collected form fields
    Device 5 – Scale  (XML)       – raw XML body

Puller flow (multi-trigger):
  Adapter processes Device 2 event
    → iterates over config["triggers"]
    → publishes {trigger_source_id: dev3_sid, gate_id: GATE_1, ...}
      to events:puller stream (one message per trigger entry)
  Puller reads trigger_source_id → fetches Device 3's config from Core → reads trigger_url
    → GETs trigger_url
    → POSTs result to Ingestor as Device 3, including original transaction_id

Matchmaker rules:
  – Normal path:  find active Valkey key for gate → reuse or create new transaction;
                  enforce max_events_per_transaction (rotate when full).
  – Puller path:  validate external transaction_id (exists in DB + correct gate)
                  → attach event; fallback to new transaction if invalid.

Tests:
  2.1  Camera ingest (multipart + image)              — raw_payload + gate_id + type_code verified
  2.2  Puller config — triggers array on Device 2     — URL + source_id in triggers verified
  2.3  Scale ingest + automatic puller trigger        — raw_payload + gate_id + type_code verified
  2.4  Manual trigger via Core API                    — all trigger entries queued + type_code
  2.5  Raw JSON body (application/json)               — raw_payload + gate_id + type_code verified
  2.6  Auto-collected form fields (no payload field)  — raw_payload + gate_id + type_code verified
  2.7  Raw XML body (application/xml)                 — raw_payload + gate_id + type_code verified
  2.8  Transaction isolation (GATE_1 ≠ GATE_2)        — transaction_id isolation
  2.9  Matchmaker — external transaction_id (Puller)  — attach valid / reject invalid + type_code
  2.10 BeforeSave hook                                — type_code + searchable_value populated
  2.11 Vehicle history — exact plate search           — GET /transactions/history returns results
  2.12 Vehicle history — fuzzy/no-match               — Levenshtein distance + true negative
  2.13 searchable_key CRUD                            — GET returns it; PUT clears/restores; hook respects it

Usage:
    ADMIN_DEFAULT_PASSWORD=secret python test.py          # run / reuse env
    ADMIN_DEFAULT_PASSWORD=secret python test.py --reset  # wipe cache, recreate
    ADMIN_DEFAULT_PASSWORD=secret python test.py --much N # stress test N events

Dependencies:
    pip install requests redis
"""
import argparse
import json
import os
import sys
import time
import random

import requests
from redis import Redis

# ─── Connection ───────────────────────────────────────────────────────────────
BASE_URL   = os.getenv("BASE_URL",   "http://localhost:8090")
VALKEY_URL = os.getenv("VALKEY_URL", "redis://localhost:6380")
ADMIN_PASS = os.getenv("ADMIN_DEFAULT_PASSWORD")

GATE_1        = "gate-north"        # Dev1, Dev2, Dev3
GATE_2        = "gate-south"        # Dev4, Dev5
GATE_HISTORY  = "gate-history-test" # fuzzy-search / history tests
TRIGGER_URL   = "https://picsum.photos/400/300"  # returns a real JPEG

# Fixed plates used in history tests (consistent across re-runs)
PLATE_EXACT = "BC1234AX"   # base plate stored in DB
PLATE_FUZZY = "BC1234A0"   # distance-1 typo (X → 0) — should still match
PLATE_MISS  = "ZZZZZZZZ"   # should never match

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
    # History / fuzzy-search tests
    "type_plate_event": P + "type:plate_event:id",
    "hist_dev_sid":     P + "hist:dev:source_id",
    "hist_dev_key":     P + "hist:dev:api_key",
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
def get_or_create_event_type(rv, hdrs, code, name, fields, cache_key, searchable_key=None):
    cached = rv.get(cache_key)
    if cached:
        skip(f"Event type '{code}'  (cached id={cached})")
        if searchable_key is not None:
            requests.put(
                f"{BASE_URL}/api/v1/types/{cached}", headers=hdrs,
                json={"searchable_key": searchable_key}, timeout=10,
            )
        return cached

    all_types = requests.get(f"{BASE_URL}/api/v1/types", headers=hdrs, timeout=10).json()
    existing = next((t for t in all_types if t["code"] == code), None)
    if existing:
        rv.set(cache_key, existing["id"])
        skip(f"Event type '{code}'  (exists id={existing['id']})")
        if searchable_key is not None and existing.get("searchable_key") != searchable_key:
            requests.put(
                f"{BASE_URL}/api/v1/types/{existing['id']}", headers=hdrs,
                json={"searchable_key": searchable_key}, timeout=10,
            )
        return existing["id"]

    body = {"code": code, "name": name, "fields": fields}
    if searchable_key is not None:
        body["searchable_key"] = searchable_key
    r = requests.post(f"{BASE_URL}/api/v1/types", headers=hdrs, json=body, timeout=10)
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
    """Create or update a device config to match the expected settings.

    Keyword args map directly to JSON fields: data_type, data_mapping,
    trigger_enabled, triggers (list of {url, source_id} dicts).
    """
    body = {
        "source_id":     source_id,
        "event_type_id": event_type_id,
        "gate_id":       gate_id,
        **kw,
    }
    # Normalise triggers: always send an explicit list (never null)
    if "triggers" not in body:
        body["triggers"] = []

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
      2  – Scale   – trigger_enabled, triggers=[{source_id: dev3}]
      3  – Camera  – puller pull target; trigger_url=TRIGGER_URL lives on THIS device's config

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
    # Device 1 – camera, no trigger
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
        triggers=[],
    )
    # Device 2 – scale, multi-trigger: targets Device 3 (source_id only; URL lives on Device 3)
    dev2_cfg_id = ensure_device_config(
        hdrs, dev2_sid, scale_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={"weight_kg": "$.Payload.Measurements.Weight.Value"},
        trigger_enabled=True,
        triggers=[{"source_id": dev3_sid}],
    )
    # Device 3 – camera, puller target; Puller resolves trigger_url from THIS device's config
    ensure_device_config(
        hdrs, dev3_sid, cam_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={
            "plate":      "$.Event.Data.Content.VideoResult.plate.text",
            "confidence": "$.Event.Data.Content.VideoResult.confidence",
        },
        trigger_enabled=False,
        triggers=[],
        trigger_url=TRIGGER_URL,
    )
    # Device 4 – flat JSON scale
    ensure_device_config(
        hdrs, dev4_sid, flat_scale_type_id,
        gate_id=GATE_2,
        data_type="json",
        data_mapping={"weight_kg": "$.weight_kg"},
        trigger_enabled=False,
        triggers=[],
    )
    # Device 5 – XML scale
    ensure_device_config(
        hdrs, dev5_sid, xml_scale_type_id,
        gate_id=GATE_2,
        data_type="xml",
        data_mapping={"weight_kg": "$.scale.weight_kg"},
        trigger_enabled=False,
        triggers=[],
    )

    return (
        dev1_sid, dev1_key,
        dev2_sid, dev2_key, dev2_cfg_id,
        dev3_sid,
        dev4_sid, dev4_key,
        dev5_sid, dev5_key,
        cam_type_id,
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
    actual = ev.get("gate_id")
    if actual != expected_gate:
        fail(f"'{label}': gate_id mismatch — expected '{expected_gate}', got '{actual}'")
    ok(f"gate_id correct  (gate_id={actual})")


def assert_type_code(ev, expected_code, label):
    """
    Перевіряє, що BeforeSave-хук заповнив type_code.
    Це поле денормалізує EventType.Code в саму таблицю events,
    щоб уникати JOIN при пошуку.
    """
    actual = ev.get("type_code", "")
    if actual != expected_code:
        fail(
            f"'{label}': type_code mismatch — expected '{expected_code}', got '{actual}'\n"
            f"         Check: BeforeSave hook on Event model, pg extension not needed for this field"
        )
    ok(f"type_code = '{actual}'  (BeforeSave hook correct)")


def assert_searchable_value(ev, expected_plate, label):
    """
    Перевіряє searchable_value — матеріалізоване нормалізоване поле
    для нечіткого пошуку. Заповнюється BeforeSave хуком з data[searchable_key].
    """
    expected = expected_plate.upper().replace(" ", "")
    actual   = ev.get("searchable_value", "")
    if actual != expected:
        fail(
            f"'{label}': searchable_value mismatch — expected '{expected}', got '{actual}'\n"
            f"         Check: BeforeSave reads data[searchable_key] and normalises it"
        )
    ok(f"searchable_value = '{actual}'  (value normalised correctly)")


def assert_searchable_empty(ev, label):
    """
    Перевіряє, що searchable_value порожній для подій, чий EventType не має searchable_key.
    Захищає від помилкового заповнення BeforeSave-хука.
    """
    actual = ev.get("searchable_value", "")
    if actual:
        fail(
            f"'{label}': searchable_value should be empty for event type with no searchable_key, got '{actual}'\n"
            f"         Check: BeforeSave must skip events when EventType.searchable_key is empty"
        )
    ok(f"searchable_value empty (EventType has no searchable_key — hook guard correct)")


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
    assert_type_code(ev, "camera_recognition", "Device 1 camera")
    assert_searchable_empty(ev, "Device 1 camera")


# ─── Test 2.2 – Puller config — triggers array on Device 2 ───────────────────
def test_puller_config_resolution(hdrs, dev2_sid, dev3_sid):
    step("Test 2.2  ·  Puller Config — triggers[] on Device 2 + trigger_url on Device 3")

    # Device 2 must have triggers[] pointing at Device 3's source_id (no url in entry)
    r2 = requests.get(
        f"{BASE_URL}/api/v1/configs/devices/{dev2_sid}", headers=hdrs, timeout=10
    )
    if r2.status_code != 200:
        fail(f"Cannot fetch Device 2 config ({r2.status_code}): {r2.text}")

    cfg2 = r2.json()
    triggers = cfg2.get("triggers") or []

    if not triggers:
        fail(f"Device 2 config has no triggers  (got: {cfg2})")

    trigger = triggers[0]
    if trigger.get("source_id") != dev3_sid:
        fail(
            f"Device 2 trigger[0].source_id mismatch: "
            f"expected '{dev3_sid}', got '{trigger.get('source_id')}'"
        )
    if cfg2.get("gate_id") != GATE_1:
        fail(f"Device 2 config has wrong gate_id: expected '{GATE_1}', got '{cfg2.get('gate_id')}'")

    ok(f"Device 2 triggers[0]: source_id='{trigger['source_id']}'  (no url — correct)")
    ok(f"gate_id correct  (gate_id='{cfg2.get('gate_id')}')")
    info(f"Adapter will iterate {len(triggers)} trigger(s) and publish one Puller task each")

    # Device 3 must have trigger_url set — Puller reads it from here
    r3 = requests.get(
        f"{BASE_URL}/api/v1/configs/devices/{dev3_sid}", headers=hdrs, timeout=10
    )
    if r3.status_code != 200:
        fail(f"Cannot fetch Device 3 config ({r3.status_code}): {r3.text}")

    cfg3 = r3.json()
    if not (cfg3.get("trigger_url") or "").strip():
        fail(f"Device 3 config is missing trigger_url — Puller cannot pull it  (got: {cfg3})")

    ok(f"Device 3 trigger_url present: '{cfg3['trigger_url']}'")


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

    ingest_tx_id = r.json().get("transaction_id")
    ok(f"Ingestor accepted  (transaction_id={ingest_tx_id})")

    ev2 = wait_for_new_event(hdrs, dev2_sid, before_dev2, "Device 2 scale", timeout=20)
    assert_raw_payload(ev2, "Device 2 scale", expected_fragment="25400.0")
    assert_gate_id(ev2, GATE_1, "Device 2 scale")
    assert_type_code(ev2, "scale_weight", "Device 2 scale")
    assert_searchable_empty(ev2, "Device 2 scale")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before_dev3, "Device 3 (puller auto)", timeout=35)
    assert_raw_payload(ev3, "Device 3 (puller auto)")
    # gate_id must be GATE_1, not "system" (the Puller's own API-key gate).
    assert_gate_id(ev3, GATE_1, "Device 3 (puller auto)")
    assert_type_code(ev3, "camera_recognition", "Device 3 (puller auto)")
    assert_searchable_empty(ev3, "Device 3 (puller auto)")

    # Both events should be in the same transaction (Puller passes the original tx_id)
    tx2 = ev2.get("transaction_id")
    tx3 = ev3.get("transaction_id")
    if tx2 and tx3 and tx2 == tx3:
        ok(f"Both events share the same transaction  (tx={tx2[:8]}…)")
    else:
        warn(f"Transactions differ: dev2 tx={tx2}, dev3 tx={tx3}  (TTL may have expired)")


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

    resp = r.json()
    ok(f"Manual trigger accepted  (response={resp})")
    # Response message should mention how many triggers were queued
    queued_msg = resp.get("message", "")
    info(f"Core response: '{queued_msg}'")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before_dev3, "Device 3 (manual trigger)", timeout=35)
    assert_raw_payload(ev3, "Device 3 (manual trigger)")
    assert_gate_id(ev3, GATE_1, "Device 3 (manual trigger)")
    assert_type_code(ev3, "camera_recognition", "Device 3 (manual trigger)")
    assert_searchable_empty(ev3, "Device 3 (manual trigger)")


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
    assert_type_code(ev, "camera_recognition", "Device 1 raw JSON body")
    assert_searchable_empty(ev, "Device 1 raw JSON body")


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
    assert_raw_payload(ev, "Device 4 auto form fields", expected_fragment="weight_kg")
    assert_gate_id(ev, GATE_2, "Device 4 auto form fields")
    assert_type_code(ev, "scale_weight_flat", "Device 4 auto form fields")
    assert_searchable_empty(ev, "Device 4 auto form fields")


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
    assert_type_code(ev, "scale_weight_xml", "Device 5 raw XML body")
    assert_searchable_empty(ev, "Device 5 raw XML body")


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


# ─── Test 2.9 – Matchmaker: external transaction_id (Puller path) ─────────────
def test_matchmaker_external_transaction(hdrs, dev1_sid, cam_type_id):
    step("Test 2.9  ·  Matchmaker — external transaction_id  (Puller path validation)")

    # ── Part A: valid external transaction_id → event must attach to it ────────
    ev_existing = get_latest_event(hdrs, dev1_sid)
    if not ev_existing:
        fail("No GATE_1 events to derive a transaction_id from — run test 2.1 first")

    tx_id = ev_existing.get("transaction_id")
    if not tx_id:
        fail(f"Latest GATE_1 event has no transaction_id  (event id={ev_existing.get('id')})")

    # Count events currently in that transaction
    r_tx = requests.get(f"{BASE_URL}/api/v1/transactions/{tx_id}", headers=hdrs, timeout=10)
    if r_tx.status_code != 200:
        fail(f"Cannot fetch transaction {tx_id}: {r_tx.text}")
    events_before = len(r_tx.json().get("events") or [])

    # POST event directly to Core with that transaction_id (simulates Puller re-inject)
    r_ev = requests.post(
        f"{BASE_URL}/api/v1/events",
        headers=hdrs,
        json={
            "event_type_id": cam_type_id,
            "gate_id":       GATE_1,
            "source_id":     dev1_sid,
            "data":          {"plate": "MATCHMAKER-TEST"},
            "transaction_id": tx_id,
        },
        timeout=10,
    )
    if r_ev.status_code not in (200, 201):
        fail(f"Cannot POST event with external tx_id ({r_ev.status_code}): {r_ev.text}")

    resp_ev = r_ev.json()
    returned_tx = resp_ev.get("transaction_id")
    if str(returned_tx) != str(tx_id):
        fail(
            f"Matchmaker ignored valid external transaction_id!\n"
            f"  expected: {tx_id}\n"
            f"  got:      {returned_tx}"
        )
    ok(f"Valid external tx_id honoured — event attached  (tx={tx_id[:8]}…)")

    # BeforeSave має виставити type_code навіть для подій, створених напряму в Core
    direct_ev = resp_ev.get("event", {})
    assert_type_code(direct_ev, "camera_recognition", "Matchmaker Part A (direct Core POST)")
    assert_searchable_empty(direct_ev, "Matchmaker Part A (direct Core POST)")

    # Verify count in that specific transaction increased (no inflation to a new one)
    r_tx2 = requests.get(f"{BASE_URL}/api/v1/transactions/{tx_id}", headers=hdrs, timeout=10)
    events_after = len(r_tx2.json().get("events") or [])
    if events_after <= events_before:
        fail(
            f"Event was NOT added to the target transaction!\n"
            f"  transaction {tx_id[:8]}… had {events_before} events, now {events_after}"
        )
    ok(f"Event count in transaction: {events_before} → {events_after}  (no false new transaction)")

    # ── Part B: invalid / non-existent transaction_id → must create a new one ──
    nil_tx = "00000000-0000-0000-0000-000000000000"
    r_bad = requests.post(
        f"{BASE_URL}/api/v1/events",
        headers=hdrs,
        json={
            "event_type_id": cam_type_id,
            "gate_id":       GATE_1,
            "source_id":     dev1_sid,
            "data":          {"plate": "FAKE-TX-TEST"},
            "transaction_id": nil_tx,
        },
        timeout=10,
    )
    if r_bad.status_code not in (200, 201):
        fail(f"Cannot POST event with nil tx_id ({r_bad.status_code}): {r_bad.text}")

    resp_bad   = r_bad.json()
    fallback_tx = resp_bad.get("transaction_id")
    if str(fallback_tx) == nil_tx:
        fail(f"Matchmaker used non-existent nil transaction_id — must fall back to a new one")
    ok(f"Invalid tx_id correctly rejected → new transaction created  (new_tx={str(fallback_tx)[:8]}…)")
    assert_type_code(resp_bad.get("event", {}), "camera_recognition", "Matchmaker Part B (nil tx)")

    # ── Part C: wrong-gate transaction_id → must create a new one ─────────────
    # Grab a GATE_2 transaction id (from dev4 if available)
    r_txs = requests.get(
        f"{BASE_URL}/api/v1/transactions?gate_id={GATE_2}&limit=1", headers=hdrs, timeout=10
    )
    if r_txs.status_code == 200:
        tx_data = r_txs.json()
        items = tx_data.get("data") or tx_data if isinstance(tx_data, list) else []
        if items:
            wrong_gate_tx = items[0].get("id") or items[0].get("transaction_id")
            if wrong_gate_tx:
                r_cross = requests.post(
                    f"{BASE_URL}/api/v1/events",
                    headers=hdrs,
                    json={
                        "event_type_id": cam_type_id,
                        "gate_id":       GATE_1,
                        "source_id":     dev1_sid,
                        "data":          {"plate": "WRONG-GATE-TX"},
                        "transaction_id": wrong_gate_tx,
                    },
                    timeout=10,
                )
                if r_cross.status_code in (200, 201):
                    resp_cross = r_cross.json()
                    cross_tx = resp_cross.get("transaction_id")
                    if str(cross_tx) == str(wrong_gate_tx):
                        fail(
                            f"Matchmaker accepted a cross-gate transaction_id!\n"
                            f"  GATE_2 tx {wrong_gate_tx[:8]}… was used for a GATE_1 event"
                        )
                    ok(f"Cross-gate tx_id rejected → new transaction created  (new_tx={str(cross_tx)[:8]}…)")
                    assert_type_code(resp_cross.get("event", {}), "camera_recognition", "Matchmaker Part C (cross-gate)")
                    return
    info("Part C skipped — no GATE_2 transactions available yet (run tests 2.6–2.7 first)")


# ─── History / fuzzy-search setup ────────────────────────────────────────────
def setup_history_env(rv, hdrs):
    """
    Ідемпотентне налаштування для тестів нечіткого пошуку:
      - Gate GATE_HISTORY з max_events_per_transaction=1
        (після першої події транзакція автоматично ротується → стає «закритою»)
      - EventType "PlateEvent" з полем plate
      - Тестовий пристрій на GATE_HISTORY
    Повертає (plate_event_type_id, hist_dev_sid).
    """
    step("History Test Environment")

    # Gate
    all_gates = requests.get(f"{BASE_URL}/api/v1/gates", headers=hdrs, timeout=10).json()
    if not any(g.get("gate_id") == GATE_HISTORY for g in all_gates):
        r = requests.post(
            f"{BASE_URL}/api/v1/gates", headers=hdrs,
            json={
                "gate_id": GATE_HISTORY,
                "name":    "History Test Gate",
                "settings": {"max_events_per_transaction": 1},
            },
            timeout=10,
        )
        if r.status_code != 201:
            fail(f"Cannot create gate '{GATE_HISTORY}': {r.text}")
        ok(f"Created gate '{GATE_HISTORY}'")
    else:
        skip(f"Gate '{GATE_HISTORY}' already exists")

    # EventType "PlateEvent"
    plate_event_type_id = get_or_create_event_type(
        rv, hdrs,
        code="PlateEvent",
        name="Plate Event",
        fields={"plate": {"type": "string", "required": True}},
        cache_key=K["type_plate_event"],
        searchable_key="plate",
    )

    # API key + device config
    hist_dev_sid, _ = get_or_create_api_key(
        rv, hdrs, K["hist_dev_sid"], K["hist_dev_key"],
        name="Test – History Device",
        gate_id=GATE_HISTORY,
        perms=["ingest:events"],
    )
    ensure_device_config(
        hdrs, hist_dev_sid, plate_event_type_id,
        gate_id=GATE_HISTORY,
        data_type="json",
        data_mapping={"plate": "$.plate"},
        trigger_enabled=False,
        triggers=[],
    )

    return plate_event_type_id, hist_dev_sid


def _post_plate_event_to_core(hdrs, plate_event_type_id, hist_dev_sid, plate):
    """
    Надсилає PlateEvent напряму в Core (минаючи Ingestor/Adapter),
    щоб протестувати BeforeSave-хук ізольовано.
    Повертає JSON-відповідь {'event': {...}, 'transaction_id': '...'}.
    """
    r = requests.post(
        f"{BASE_URL}/api/v1/events",
        headers=hdrs,
        json={
            "event_type_id": plate_event_type_id,
            "gate_id":       GATE_HISTORY,
            "source_id":     hist_dev_sid,
            "data":          {"plate": plate},
        },
        timeout=10,
    )
    if r.status_code not in (200, 201):
        fail(f"Cannot POST PlateEvent to Core ({r.status_code}): {r.text}")
    return r.json()


def _close_history_gate_transaction(rv):
    """
    Видаляє активний ключ транзакції для GATE_HISTORY з Valkey.
    Після цього annotateOpen() вважає всі транзакції цього шлагбаума закритими,
    і FindPastTransactionsFuzzy повертає їх у результатах.
    """
    key = f"tx_active:{GATE_HISTORY}"
    rv.delete(key)
    info(f"Deleted Valkey key '{key}' — transaction now appears closed")


# ─── Test 2.10 – BeforeSave hook: searchable_value та type_code заповнюються ──
def test_searchable_value_hook(hdrs, plate_event_type_id, hist_dev_sid):
    step("Test 2.10  ·  BeforeSave Hook — searchable_value та type_code")

    resp = _post_plate_event_to_core(hdrs, plate_event_type_id, hist_dev_sid, PLATE_EXACT)
    ev = resp.get("event", {})

    type_code = ev.get("type_code", "")
    searchable = ev.get("searchable_value", "")
    expected = PLATE_EXACT.upper().replace(" ", "")

    if type_code != "PlateEvent":
        fail(
            f"type_code not set correctly by BeforeSave\n"
            f"  expected: 'PlateEvent'\n"
            f"  got:      '{type_code}'"
        )
    ok(f"type_code = '{type_code}'  (correct)")

    if searchable != expected:
        fail(
            f"searchable_value not set correctly by BeforeSave\n"
            f"  expected: '{expected}'\n"
            f"  got:      '{searchable}'"
        )
    ok(f"searchable_value = '{searchable}'  (normalized plate, correct)")
    info(f"event id={ev.get('id')}  tx={resp.get('transaction_id', '')[:8]}…")


# ─── Test 2.11 & 2.12 – GET /transactions/history: exact та fuzzy match ───────
def test_vehicle_history_search(hdrs, rv, plate_event_type_id, hist_dev_sid):
    step("Test 2.11/2.12  ·  Vehicle History Search  (exact + fuzzy + no-match)")

    # Переконуємось, що в Core є хоча б один PlateEvent з PLATE_EXACT
    # (test 2.10 міг вже його створити; якщо ні — створюємо зараз)
    _post_plate_event_to_core(hdrs, plate_event_type_id, hist_dev_sid, PLATE_EXACT)

    # «Закриваємо» транзакцію: видаляємо активний ключ з Valkey.
    # FindPastTransactionsFuzzy фільтрує відкриті транзакції через annotateOpen();
    # без цього кроку пошук поверне порожній список.
    _close_history_gate_transaction(rv)

    # ── Part A: точний збіг ──────────────────────────────────────────────────
    r_exact = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_EXACT}",
        headers=hdrs,
        timeout=10,
    )
    if r_exact.status_code != 200:
        fail(f"History endpoint failed for exact match ({r_exact.status_code}): {r_exact.text}")

    body_exact = r_exact.json()
    found = body_exact.get("data") or []
    if not found:
        fail(
            f"Exact search returned 0 results for plate '{PLATE_EXACT}'\n"
            f"  Check: pg_trgm/fuzzystrmatch extensions enabled, GIN index created, BeforeSave runs"
        )
    ok(f"Exact match: found {len(found)} transaction(s) for plate '{PLATE_EXACT}'")

    # Перевіряємо, що у знайденій транзакції є події
    has_events = any(len(tx.get("events") or []) > 0 for tx in found)
    if not has_events:
        warn("Transactions returned without events — check Preload(\"Events\") in FindPastTransactionsFuzzy")
    else:
        ok("Transactions include preloaded events  (images accessible)")

    # ── Part B: нечіткий збіг (відстань Левенштейна = 1) ────────────────────
    r_fuzzy = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_FUZZY}",
        headers=hdrs,
        timeout=10,
    )
    if r_fuzzy.status_code != 200:
        fail(f"History endpoint failed for fuzzy match ({r_fuzzy.status_code}): {r_fuzzy.text}")

    found_fuzzy = r_fuzzy.json().get("data") or []
    if not found_fuzzy:
        fail(
            f"Fuzzy search returned 0 results for plate '{PLATE_FUZZY}' "
            f"(expected to match '{PLATE_EXACT}', distance=1)\n"
            f"  Check: levenshtein_less_equal() in SQL, fuzzystrmatch extension enabled"
        )
    ok(
        f"Fuzzy match (distance=1): found {len(found_fuzzy)} transaction(s) "
        f"for plate '{PLATE_FUZZY}' → matched '{PLATE_EXACT}'"
    )

    # ── Part C: гарантований промах ─────────────────────────────────────────
    r_miss = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_MISS}",
        headers=hdrs,
        timeout=10,
    )
    if r_miss.status_code != 200:
        fail(f"History endpoint failed for no-match ({r_miss.status_code}): {r_miss.text}")

    found_miss = r_miss.json().get("data") or []
    if found_miss:
        warn(
            f"No-match search for '{PLATE_MISS}' returned {len(found_miss)} result(s) — "
            f"possibly stale data in DB with a similar plate"
        )
    else:
        ok(f"No-match plate '{PLATE_MISS}' correctly returns 0 results")


# ─── Test 2.13 – GET /types returns searchable_key; PUT /types/:id updates it ─
def test_searchable_key_crud(hdrs, plate_event_type_id):
    step("Test 2.13  ·  searchable_key — GET returns it, PUT updates it")

    # Part A: verify GET /types returns searchable_key = "plate" for PlateEvent
    r = requests.get(f"{BASE_URL}/api/v1/types", headers=hdrs, timeout=10)
    if r.status_code != 200:
        fail(f"GET /types failed ({r.status_code}): {r.text}")
    all_types = r.json()
    plate_type = next((t for t in all_types if t["id"] == plate_event_type_id), None)
    if plate_type is None:
        fail(f"PlateEvent type (id={plate_event_type_id}) not found in GET /types response")
    sk = plate_type.get("searchable_key", "")
    if sk != "plate":
        fail(
            f"GET /types: searchable_key mismatch for PlateEvent\n"
            f"  expected: 'plate'\n"
            f"  got:      '{sk}'"
        )
    ok(f"GET /types: searchable_key = '{sk}'  (correct)")

    # Part B: clear searchable_key via PUT and verify hook no longer populates searchable_value
    r_clear = requests.put(
        f"{BASE_URL}/api/v1/types/{plate_event_type_id}",
        headers=hdrs,
        json={"searchable_key": ""},
        timeout=10,
    )
    if r_clear.status_code not in (200, 204):
        fail(f"PUT /types/:id (clear searchable_key) failed ({r_clear.status_code}): {r_clear.text}")
    ok("PUT /types/:id: searchable_key cleared to ''")

    r_ev = requests.post(
        f"{BASE_URL}/api/v1/events",
        headers=hdrs,
        json={
            "event_type_id": plate_event_type_id,
            "gate_id":       GATE_HISTORY,
            "source_id":     "test-probe",
            "data":          {"plate": PLATE_EXACT},
        },
        timeout=10,
    )
    if r_ev.status_code not in (200, 201):
        fail(f"POST /events (probe) failed ({r_ev.status_code}): {r_ev.text}")
    ev_probe = r_ev.json().get("event", {})
    sv = ev_probe.get("searchable_value", "")
    if sv:
        fail(
            f"searchable_value should be empty after clearing searchable_key, got '{sv}'\n"
            f"  Check: BeforeSave reads et.SearchableKey at save time, not cached"
        )
    ok(f"searchable_value empty after clearing searchable_key  (hook respects DB config)")

    # Part C: restore searchable_key = "plate" so subsequent tests still work
    r_restore = requests.put(
        f"{BASE_URL}/api/v1/types/{plate_event_type_id}",
        headers=hdrs,
        json={"searchable_key": "plate"},
        timeout=10,
    )
    if r_restore.status_code not in (200, 204):
        fail(f"PUT /types/:id (restore searchable_key) failed ({r_restore.status_code}): {r_restore.text}")
    ok("PUT /types/:id: searchable_key restored to 'plate'")


# ─── Gate helpers for load test ───────────────────────────────────────────────
def get_or_create_gate(hdrs, gate_id, name, settings):
    all_gates = requests.get(f"{BASE_URL}/api/v1/gates", headers=hdrs, timeout=10).json()
    existing = next((g for g in all_gates if g.get("gate_id") == gate_id), None)
    if existing:
        ur = requests.put(
            f"{BASE_URL}/api/v1/gates/{existing['id']}", headers=hdrs,
            json={"gate_id": gate_id, "name": name, "settings": settings}, timeout=10
        )
        if ur.status_code not in (200, 204):
            fail(f"Cannot update gate '{gate_id}': {ur.text}")
        skip(f"Gate '{gate_id}'  (exists id={existing['id']})")
        return existing["id"]

    r = requests.post(
        f"{BASE_URL}/api/v1/gates", headers=hdrs,
        json={"gate_id": gate_id, "name": name, "settings": settings},
        timeout=10,
    )
    if r.status_code != 201:
        fail(f"Cannot create gate '{gate_id}': {r.text}")
    gate_uuid = r.json().get("id")
    ok(f"Created gate '{gate_id}'  (id={gate_uuid})")
    return gate_uuid


# ─── Load test ────────────────────────────────────────────────────────────────
def run_much(rv, hdrs, num_events):
    step(f"Load Test  ·  Generating {num_events} events")

    GATE_MUCH = "gate-much"
    get_or_create_gate(
        hdrs, GATE_MUCH, "Load Test Gate",
        {"transaction_ttl_minutes": 1, "max_events_per_transaction": 5}
    )

    cam_type_id = get_or_create_event_type(
        rv, hdrs, code="camera_much", name="Camera (Much)",
        fields={"plate": {"type": "string", "required": True}},
        cache_key="omnigate:test:type_camera_much",
    )
    scale_type_id = get_or_create_event_type(
        rv, hdrs, code="scale_much", name="Scale (Much)",
        fields={"weight_kg": {"type": "float", "required": True}},
        cache_key="omnigate:test:type_scale_much",
    )

    # Puller target: trigger_url is set here; scale devices reference this source_id in their triggers[]
    puller_sid, puller_key = get_or_create_api_key(
        rv, hdrs, "omnigate:test:much_puller_sid", "omnigate:test:much_puller_key",
        name="Load Puller Target", gate_id=GATE_MUCH, perms=["ingest:events"]
    )
    ensure_device_config(
        hdrs, puller_sid, cam_type_id,
        gate_id=GATE_MUCH, data_type="json",
        data_mapping={"plate": "$.plate"},
        trigger_enabled=False,
        triggers=[],
        trigger_url=TRIGGER_URL,
    )

    devs = []
    for i in range(5):
        is_cam = (i % 2 == 0)
        sid, key = get_or_create_api_key(
            rv, hdrs, f"omnigate:test:much_dev{i}_sid", f"omnigate:test:much_dev{i}_key",
            name=f"Load Dev {i}", gate_id=GATE_MUCH, perms=["ingest:events"]
        )
        # Scale devices trigger the puller target; camera devices have no trigger
        ensure_device_config(
            hdrs, sid, cam_type_id if is_cam else scale_type_id,
            gate_id=GATE_MUCH, data_type="json",
            data_mapping={"plate": "$.plate"} if is_cam else {"weight_kg": "$.weight"},
            trigger_enabled=not is_cam,
            triggers=[{"source_id": puller_sid}] if not is_cam else [],
        )
        devs.append((sid, key, is_cam))

    info(f"Starting to send {num_events} events...")
    start_t = time.time()
    for n in range(1, num_events + 1):
        sid, key, is_cam = random.choice(devs)
        payload = {"plate": f"AA{random.randint(1000, 9999)}BB"} if is_cam \
                  else {"weight": random.randint(1000, 40000)}

        r = requests.post(
            f"{BASE_URL}/ingest/event",
            headers={"X-API-Key": key},
            json=payload,
            timeout=5
        )
        if r.status_code != 202:
            warn(f"Event {n} failed: {r.status_code} {r.text}")
        elif n % 50 == 0 or n == num_events:
            print(f"  ... sent {n}/{num_events} events", end="\r")

    dur = time.time() - start_t
    print()
    ok(f"Generated {num_events} events in {dur:.2f}s ({num_events/dur:.1f} ev/s)")


# ─── Main ─────────────────────────────────────────────────────────────────────
def main():
    if not ADMIN_PASS:
        fail("ADMIN_DEFAULT_PASSWORD env var is not set")

    parser = argparse.ArgumentParser(description="OmniGate integration test")
    parser.add_argument(
        "--reset", action="store_true",
        help="Clear Valkey test cache and recreate all devices/configs",
    )
    parser.add_argument(
        "--much", type=int, metavar="N",
        help="Generate N events across multiple devices to stress test transaction limits",
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

    if args.much:
        run_much(rv, hdrs, args.much)
        return

    step("Environment Setup")
    (
        dev1_sid, dev1_key,
        dev2_sid, dev2_key, dev2_cfg_id,
        dev3_sid,
        dev4_sid, dev4_key,
        dev5_sid, dev5_key,
        cam_type_id,
    ) = setup(rv, hdrs)

    # ── GATE_1 tests ───────────────────────────────────────────────────────
    test_camera_ingest(hdrs, dev1_sid, dev1_key)
    test_puller_config_resolution(hdrs, dev2_sid, dev3_sid)
    test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid)
    test_manual_trigger(hdrs, dev2_cfg_id, dev3_sid)
    test_raw_json_body(hdrs, dev1_sid, dev1_key)

    # ── GATE_2 tests ───────────────────────────────────────────────────────
    test_auto_form_fields(hdrs, dev4_sid, dev4_key)
    test_raw_xml_body(hdrs, dev5_sid, dev5_key)

    # ── Cross-gate regression ──────────────────────────────────────────────
    test_transaction_isolation(hdrs, dev1_sid, dev4_sid)

    # ── Matchmaker unit-level test ─────────────────────────────────────────
    test_matchmaker_external_transaction(hdrs, dev1_sid, cam_type_id)

    # ── Vehicle history / fuzzy-search tests ───────────────────────────────
    plate_event_type_id, hist_dev_sid = setup_history_env(rv, hdrs)
    test_searchable_value_hook(hdrs, plate_event_type_id, hist_dev_sid)
    test_vehicle_history_search(hdrs, rv, plate_event_type_id, hist_dev_sid)
    test_searchable_key_crud(hdrs, plate_event_type_id)

    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}{GREEN}  All tests passed ✓{RESET}")
    print(f"{BOLD}{'=' * 60}{RESET}\n")


if __name__ == "__main__":
    main()
