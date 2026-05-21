#!/usr/bin/env python3
"""
OmniGate integration test.

Device layout:
  GATE_1 (gate-north):
    Device 1 – Camera  – multipart ingest with image, no trigger
    Device 2 – Scale   – JSON ingest, triggers puller → Device 3
    Device 3 – Camera  – puller pull target (trigger_url lives here)
    Device 6 – Camera  – ITSAPI/Digest Auth, JSON + base64 image
  GATE_2 (gate-south):
    Device 4 – Scale   – raw JSON body
    Device 5 – Scale   – raw XML body

Event types (two total — transport format is irrelevant to the type):
  PLATE_EVENT   – plate, confidence, region, vehicle_type; searchable_key=plate
  WEIGHT_EVENT  – weight_kg

Usage:
    ADMIN_DEFAULT_PASSWORD=secret python test.py          # run / reuse env
    ADMIN_DEFAULT_PASSWORD=secret python test.py --reset  # delete test resources, recreate
    ADMIN_DEFAULT_PASSWORD=secret python test.py --much N # stress test N events

Dependencies:
    pip install requests redis
    (no extra packages needed — HTTPDigestAuth ships with requests)
"""
import argparse
import base64
import json
import os
import sys
import time
import random

import requests
from requests.auth import HTTPDigestAuth
from redis import Redis

# ─── Connection ───────────────────────────────────────────────────────────────
BASE_URL   = os.getenv("BASE_URL",   "http://localhost:8090")
VALKEY_URL = os.getenv("VALKEY_URL", "redis://localhost:6380")
ADMIN_PASS = os.getenv("ADMIN_DEFAULT_PASSWORD")

# ─── ITSAPI camera credentials ────────────────────────────────────────────────
# These are registered in the Auth service via POST /admin/keys/:id/digest.
# Override via environment variables when running against a real camera account.
ITSAPI_USER     = os.getenv("ITSAPI_USER",      "cam_itsapi_01")
ITSAPI_PASSWORD = os.getenv("ITSAPI_PASSWORD",   "itsapi_secret")
# Path to a real JPEG to send as Base64. Falls back to the built-in fake JPEG.
ITSAPI_IMAGE    = os.getenv("ITSAPI_IMAGE_PATH", "test_car.jpg")

GATE_1       = "gate-north"
GATE_2       = "gate-south"
GATE_HISTORY = "gate-history-test"
TRIGGER_URL  = os.getenv("RTSP_URL")

PLATE_EXACT = "BC1234AX"
PLATE_FUZZY = "BC1234A0"   # distance-1 typo — should still match
PLATE_MISS  = "ZZZZZZZZ"   # should never match

FAKE_JPEG = (
    b"\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"
    b"\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n"
    b"\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d"
    b"\xff\xd9"
)

# ─── Valkey cache keys ────────────────────────────────────────────────────────
P = "omnigate:test:"
K = {
    "type_plate":  P + "type:plate:id",
    "type_weight": P + "type:weight:id",
    "dev1_sid": P + "dev1:source_id",  "dev1_key": P + "dev1:api_key",
    "dev2_sid": P + "dev2:source_id",  "dev2_key": P + "dev2:api_key",
    "dev3_sid": P + "dev3:source_id",  "dev3_key": P + "dev3:api_key",
    "dev4_sid": P + "dev4:source_id",  "dev4_key": P + "dev4:api_key",
    "dev5_sid": P + "dev5:source_id",  "dev5_key": P + "dev5:api_key",
    "hist_dev_sid": P + "hist:dev:source_id",
    "hist_dev_key": P + "hist:dev:api_key",
    # ITSAPI camera (Digest Auth)
    "itsapi_dev_sid": P + "itsapi:dev:source_id",
    "itsapi_dev_key": P + "itsapi:dev:api_key",
}

MUCH_KEYS = (
    ["omnigate:test:much_puller_sid", "omnigate:test:much_puller_key",
     "omnigate:test:type_plate_much", "omnigate:test:type_weight_much"]
    + [f"omnigate:test:much_dev{i}_{s}" for i in range(5) for s in ("sid", "key")]
)

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


# ─── Reset ────────────────────────────────────────────────────────────────────
def reset_all(rv, hdrs):
    """Delete all test resources from the system, then clear Valkey cache."""
    step("Reset — deleting test resources from system")

    # Collect all source_id (= API key ID) Valkey keys
    sid_keys = [K["dev1_sid"], K["dev2_sid"], K["dev3_sid"],
                K["dev4_sid"], K["dev5_sid"], K["hist_dev_sid"],
                K["itsapi_dev_sid"]]  # ITSAPI camera
    sid_keys += ["omnigate:test:much_puller_sid"]
    sid_keys += [f"omnigate:test:much_dev{i}_sid" for i in range(5)]

    for sid_key in sid_keys:
        sid = rv.get(sid_key)
        if not sid:
            continue
        # Delete device config (look up by source_id, delete by config UUID)
        r = requests.get(f"{BASE_URL}/api/v1/configs/devices/{sid}", headers=hdrs, timeout=10)
        if r.status_code == 200:
            cfg_id = r.json().get("id")
            if cfg_id:
                rd = requests.delete(
                    f"{BASE_URL}/api/v1/configs/devices/{cfg_id}", headers=hdrs, timeout=10
                )
                status = "deleted" if rd.status_code in (200, 204) else f"status={rd.status_code}"
                info(f"Device config source_id={sid}: {status}")
        # Delete API key
        rk = requests.delete(f"{BASE_URL}/api/auth/admin/keys/{sid}", headers=hdrs, timeout=10)
        status = "deleted" if rk.status_code in (200, 204) else f"status={rk.status_code}"
        info(f"API key id={sid}: {status}")

    # Delete all test-managed gates
    all_gates = requests.get(f"{BASE_URL}/api/v1/gates", headers=hdrs, timeout=10).json()
    for test_gate_id in [GATE_1, GATE_2, GATE_HISTORY, "gate-much"]:
        g = next((x for x in all_gates if x.get("gate_id") == test_gate_id), None)
        if g:
            rd = requests.delete(f"{BASE_URL}/api/v1/gates/{g['id']}", headers=hdrs, timeout=10)
            status = "deleted" if rd.status_code in (200, 204) else f"status={rd.status_code}"
            info(f"Gate '{test_gate_id}': {status}")

    # Clear Valkey
    for k in list(K.values()) + MUCH_KEYS:
        rv.delete(k)
    ok("All test resources removed — Valkey cache cleared")


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
    body = {"source_id": source_id, "event_type_id": event_type_id, "gate_id": gate_id, **kw}
    if "triggers" not in body:
        body["triggers"] = []

    r = requests.get(f"{BASE_URL}/api/v1/configs/devices/{source_id}", headers=hdrs, timeout=10)
    if r.status_code == 200:
        cfg_id = r.json().get("id")
        ur = requests.put(
            f"{BASE_URL}/api/v1/configs/devices/{cfg_id}", headers=hdrs, json=body, timeout=10
        )
        if ur.status_code not in (200, 204):
            fail(f"Cannot update device config (source_id={source_id}): {ur.text}")
        skip(f"Device config ensured  (source_id={source_id}, gate={gate_id})")
        return cfg_id

    cr = requests.post(f"{BASE_URL}/api/v1/configs/devices", headers=hdrs, json=body, timeout=10)
    if cr.status_code not in (200, 201):
        fail(f"Cannot create device config (source_id={source_id}): {cr.text}")
    cfg_id = cr.json().get("id")
    ok(f"Created device config  (source_id={source_id}, gate={gate_id})")
    return cfg_id


# ─── ITSAPI helpers ───────────────────────────────────────────────────────────
def encode_image_base64(path: str, fallback: bytes = FAKE_JPEG) -> str:
    """
    Read a JPEG file from disk and return a plain base64 string (no data-URI prefix).

    If the file is not found, logs a warning and falls back to FAKE_JPEG so the
    test can run in CI environments without real image assets.
    """
    try:
        with open(path, "rb") as fh:
            raw = fh.read()
        info(f"Loaded image '{path}'  ({len(raw)} bytes)")
    except FileNotFoundError:
        warn(f"Image file '{path}' not found — using built-in fake JPEG ({len(fallback)} bytes)")
        raw = fallback
    return base64.b64encode(raw).decode("utf-8")


def setup_itsapi_device(rv, hdrs, plate_type_id) -> str:
    """
    Register a Device 6 — ITSAPI camera that authenticates via HTTP Digest Auth.

    Steps:
      1. Create an API key (same as other devices).
      2. POST /admin/keys/:id/digest  — store HA1 = MD5(user:realm:password)
         so the Auth service can validate Digest responses without a plaintext
         password ever persisting in the database.
      3. Create DeviceConfig with image_fields=["front_image"] so the Adapter
         knows to decode the base64 blob and upload it to Garage/S3.
    """
    step("Device 6 — ITSAPI Camera (Digest Auth)")

    itsapi_dev_sid, _ = get_or_create_api_key(
        rv, hdrs,
        K["itsapi_dev_sid"], K["itsapi_dev_key"],
        name="Test – Device 6 (ITSAPI Camera, Digest)",
        gate_id=GATE_1,
        perms=["ingest:events"],
    )

    # Register Digest credentials.
    # The endpoint computes HA1 = MD5(username:realm:password) server-side and
    # stores only the hash — the plaintext password is never persisted.
    r = requests.post(
        f"{BASE_URL}/api/auth/admin/keys/{itsapi_dev_sid}/digest",
        headers=hdrs,
        json={"digest_username": ITSAPI_USER, "digest_password": ITSAPI_PASSWORD},
        timeout=10,
    )
    if r.status_code == 200:
        ok(f"Digest credentials registered  (username={ITSAPI_USER})")
    elif r.status_code == 409:
        skip(f"Digest credentials already set  (username={ITSAPI_USER})")
    else:
        warn(f"Digest credential endpoint returned {r.status_code}: {r.text}")

    # DeviceConfig:
    #   data_mapping  – JSONPath expressions that match the ITSAPI JSON payload
    #   image_fields  – tells the Adapter which mapped fields contain base64 images
    ensure_device_config(
        hdrs, itsapi_dev_sid, plate_type_id,
        gate_id=GATE_1,
        data_type="json",
        data_mapping={
            "plate":       "$.lp",
            "confidence":  "$.confidence",
            "front_image": "$.imageData",
        },
        image_fields=["front_image"],
        trigger_enabled=False,
        triggers=[],
    )

    return itsapi_dev_sid


# ─── Environment setup ────────────────────────────────────────────────────────
def setup(rv, hdrs):
    """
    Two event types shared by all devices:
      PLATE_EVENT  – cameras (Dev1, Dev3); searchable_key=plate
      WEIGHT_EVENT – scales  (Dev2, Dev4, Dev5); transport format is irrelevant

    Devices:
      Dev1 – camera, GATE_1, multipart JSON + image, no trigger
      Dev2 – scale,  GATE_1, JSON multipart, triggers Dev3
      Dev3 – camera, GATE_1, puller target (trigger_url set here)
      Dev4 – scale,  GATE_2, raw JSON body
      Dev5 – scale,  GATE_2, raw XML body
    """
    step("Gates")
    get_or_create_gate(hdrs, GATE_1, "North Gate", {"transaction_ttl_minutes": 1})
    get_or_create_gate(hdrs, GATE_2, "South Gate", {"transaction_ttl_minutes": 1})

    step("Event Types")
    plate_type_id = get_or_create_event_type(
        rv, hdrs,
        code="PLATE_EVENT",
        name="Plate Event",
        fields={
            "plate":        {"type": "string", "required": True},
            "confidence":   {"type": "number", "required": False},
            "region":       {"type": "string", "required": False},
            "vehicle_type": {"type": "string", "required": False},
        },
        cache_key=K["type_plate"],
        searchable_key="plate",
    )
    weight_type_id = get_or_create_event_type(
        rv, hdrs,
        code="WEIGHT_EVENT",
        name="Weight Event",
        fields={"weight_kg": {"type": "number", "required": True}},
        cache_key=K["type_weight"],
    )

    step("API Keys")
    dev1_sid, dev1_key = get_or_create_api_key(
        rv, hdrs, K["dev1_sid"], K["dev1_key"],
        name="Test – Device 1 (Camera)", gate_id=GATE_1, perms=["ingest:events"],
    )
    dev2_sid, dev2_key = get_or_create_api_key(
        rv, hdrs, K["dev2_sid"], K["dev2_key"],
        name="Test – Device 2 (Scale + trigger)", gate_id=GATE_1, perms=["ingest:events"],
    )
    dev3_sid, _ = get_or_create_api_key(
        rv, hdrs, K["dev3_sid"], K["dev3_key"],
        name="Test – Device 3 (Camera, puller target)", gate_id=GATE_1, perms=["ingest:events"],
    )
    dev4_sid, dev4_key = get_or_create_api_key(
        rv, hdrs, K["dev4_sid"], K["dev4_key"],
        name="Test – Device 4 (Scale, JSON body)", gate_id=GATE_2, perms=["ingest:events"],
    )
    dev5_sid, dev5_key = get_or_create_api_key(
        rv, hdrs, K["dev5_sid"], K["dev5_key"],
        name="Test – Device 5 (Scale, XML body)", gate_id=GATE_2, perms=["ingest:events"],
    )

    step("Device Configs")
    # Dev1 – camera, no trigger; data: {"plate": ..., "confidence": ..., ...}
    ensure_device_config(
        hdrs, dev1_sid, plate_type_id, gate_id=GATE_1,
        data_type="json",
        data_mapping={"plate": "$.plate", "confidence": "$.confidence",
                      "region": "$.region", "vehicle_type": "$.vehicle_type"},
        trigger_enabled=False, triggers=[],
    )
    # Dev2 – scale, triggers Dev3
    dev2_cfg_id = ensure_device_config(
        hdrs, dev2_sid, weight_type_id, gate_id=GATE_1,
        data_type="json",
        data_mapping={"weight_kg": "$.weight_kg"},
        trigger_enabled=True,
        triggers=[{"source_id": dev3_sid}],
    )
    # Dev3 – camera, puller target; trigger_url is fetched by the Puller from this config
    ensure_device_config(
        hdrs, dev3_sid, plate_type_id, gate_id=GATE_1,
        data_type="json",
        data_mapping={"plate": "$.plate", "confidence": "$.confidence"},
        trigger_enabled=False, triggers=[],
        trigger_url=TRIGGER_URL,
    )
    # Dev4 – scale, raw JSON body; same field shape as Dev2
    ensure_device_config(
        hdrs, dev4_sid, weight_type_id, gate_id=GATE_2,
        data_type="json",
        data_mapping={"weight_kg": "$.weight_kg"},
        trigger_enabled=False, triggers=[],
    )
    # Dev5 – scale, raw XML body; <event><weight_kg>…</weight_kg></event>
    ensure_device_config(
        hdrs, dev5_sid, weight_type_id, gate_id=GATE_2,
        data_type="xml",
        data_mapping={"weight_kg": "$.event.weight_kg"},
        trigger_enabled=False, triggers=[],
    )

    return (
        dev1_sid, dev1_key,
        dev2_sid, dev2_key, dev2_cfg_id,
        dev3_sid,
        dev4_sid, dev4_key,
        dev5_sid, dev5_key,
        plate_type_id,
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
    info(f"Waiting up to {timeout}s for '{label}' event …")
    deadline = time.time() + timeout
    while time.time() < deadline:
        if count_events(hdrs, source_id) > count_before:
            ev = get_latest_event(hdrs, source_id)
            ok(f"'{label}' event in Core  (id={ev.get('id')}, data={ev.get('data')})")
            return ev
        time.sleep(1)
    fail(f"Timeout: '{label}' event not found after {timeout}s")


# ─── Assertion helpers ────────────────────────────────────────────────────────
def assert_raw_data_key(ev, label, hdrs, expected_fragment=None):
    key = ev.get("raw_data_key") or ""
    if not key:
        fail(f"'{label}': raw_data_key missing  (event id={ev.get('id')})")
    ok(f"raw_data_key present  ({key})")
    if expected_fragment:
        event_id = ev.get("id")
        r = requests.get(f"{BASE_URL}/api/v1/events/{event_id}/raw", headers=hdrs, timeout=10)
        if r.status_code != 200:
            fail(f"'{label}': GET /events/{event_id}/raw → {r.status_code}: {r.text[:200]}")
        if expected_fragment not in r.text:
            fail(
                f"'{label}': raw content missing '{expected_fragment}'\n"
                f"         snippet: {r.text[:300]}"
            )
        ok(f"raw content verified  (contains {repr(expected_fragment)})")


def assert_gate_id(ev, expected_gate, label):
    actual = ev.get("gate_id")
    if actual != expected_gate:
        fail(f"'{label}': gate_id — expected '{expected_gate}', got '{actual}'")
    ok(f"gate_id correct  ({actual})")


def assert_type_code(ev, expected_code, label):
    actual = ev.get("type_code", "")
    if actual != expected_code:
        fail(f"'{label}': type_code — expected '{expected_code}', got '{actual}'")
    ok(f"type_code = '{actual}'")


def assert_searchable_value(ev, expected_plate, label):
    expected = expected_plate.upper().replace(" ", "")
    actual   = ev.get("searchable_value", "")
    if actual != expected:
        fail(f"'{label}': searchable_value — expected '{expected}', got '{actual}'")
    ok(f"searchable_value = '{actual}'")


# ─── Test 2.1 – Camera ingest (Device 1, multipart + image) ──────────────────
def test_camera_ingest(hdrs, dev1_sid, dev1_key):
    step("Test 2.1  ·  Camera Ingest  (Device 1 — multipart + image)")

    before = count_events(hdrs, dev1_sid)
    payload = json.dumps({
        "plate": "AA1234BB", "confidence": 0.992,
        "region": "UA", "vehicle_type": "truck",
    })
    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        data={"payload": payload},
        files={"image": ("plate.jpg", FAKE_JPEG, "image/jpeg")},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted  (transaction_id={r.json().get('transaction_id')})")

    ev = wait_for_new_event(hdrs, dev1_sid, before, "Device 1 camera")
    assert_raw_data_key(ev, "Device 1 camera", hdrs, expected_fragment="AA1234BB")
    assert_gate_id(ev, GATE_1, "Device 1 camera")
    assert_type_code(ev, "PLATE_EVENT", "Device 1 camera")


# ─── Test 2.2 – Puller config: triggers[] on Device 2, trigger_url on Device 3 ─
def test_puller_config_resolution(hdrs, dev2_sid, dev3_sid):
    step("Test 2.2  ·  Puller Config — triggers[] on Device 2 + trigger_url on Device 3")

    r2 = requests.get(f"{BASE_URL}/api/v1/configs/devices/{dev2_sid}", headers=hdrs, timeout=10)
    if r2.status_code != 200:
        fail(f"Cannot fetch Device 2 config ({r2.status_code}): {r2.text}")
    cfg2 = r2.json()
    triggers = cfg2.get("triggers") or []
    if not triggers:
        fail(f"Device 2 has no triggers  (got: {cfg2})")
    if triggers[0].get("source_id") != dev3_sid:
        fail(f"Device 2 trigger[0].source_id — expected '{dev3_sid}', got '{triggers[0].get('source_id')}'")
    if cfg2.get("gate_id") != GATE_1:
        fail(f"Device 2 gate_id — expected '{GATE_1}', got '{cfg2.get('gate_id')}'")
    ok(f"Device 2 triggers[0].source_id = '{dev3_sid}'")
    ok(f"Device 2 gate_id = '{GATE_1}'")

    r3 = requests.get(f"{BASE_URL}/api/v1/configs/devices/{dev3_sid}", headers=hdrs, timeout=10)
    if r3.status_code != 200:
        fail(f"Cannot fetch Device 3 config ({r3.status_code}): {r3.text}")
    cfg3 = r3.json()
    if not (cfg3.get("trigger_url") or "").strip():
        fail(f"Device 3 missing trigger_url  (got: {cfg3})")
    ok(f"Device 3 trigger_url = '{cfg3['trigger_url']}'")


# ─── Test 2.3 – Scale ingest + automatic puller trigger ──────────────────────
def test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid):
    step("Test 2.3  ·  Scale Ingest + Automatic Puller Trigger  (Device 2 → Device 3)")

    before2 = count_events(hdrs, dev2_sid)
    before3 = count_events(hdrs, dev3_sid)

    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev2_key},
        data={"payload": json.dumps({"weight_kg": 25400.0})},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected event ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted  (transaction_id={r.json().get('transaction_id')})")

    ev2 = wait_for_new_event(hdrs, dev2_sid, before2, "Device 2 scale", timeout=20)
    assert_raw_data_key(ev2, "Device 2 scale", hdrs, expected_fragment="25400")
    assert_gate_id(ev2, GATE_1, "Device 2 scale")
    assert_type_code(ev2, "WEIGHT_EVENT", "Device 2 scale")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before3, "Device 3 (puller)", timeout=50)
    assert_raw_data_key(ev3, "Device 3 (puller)", hdrs)
    assert_gate_id(ev3, GATE_1, "Device 3 (puller)")
    assert_type_code(ev3, "PLATE_EVENT", "Device 3 (puller)")

    tx2, tx3 = ev2.get("transaction_id"), ev3.get("transaction_id")
    if tx2 and tx3 and tx2 == tx3:
        ok(f"Both events share the same transaction  (tx={tx2[:8]}…)")
    else:
        warn(f"Transactions differ: dev2={tx2}, dev3={tx3}  (TTL may have expired)")


# ─── Test 2.4 – Manual trigger via Core API ───────────────────────────────────
def test_manual_trigger(hdrs, dev2_cfg_id, dev3_sid):
    step("Test 2.4  ·  Manual Trigger  (POST /configs/devices/{id}/trigger)")

    before3 = count_events(hdrs, dev3_sid)
    r = requests.post(
        f"{BASE_URL}/api/v1/configs/devices/{dev2_cfg_id}/trigger",
        headers=hdrs, timeout=10,
    )
    if r.status_code not in (200, 202):
        fail(f"Manual trigger failed ({r.status_code}): {r.text}")
    ok(f"Manual trigger accepted  ({r.json().get('message', r.json())})")

    ev3 = wait_for_new_event(hdrs, dev3_sid, before3, "Device 3 (manual trigger)", timeout=50)
    assert_raw_data_key(ev3, "Device 3 (manual trigger)", hdrs)
    assert_gate_id(ev3, GATE_1, "Device 3 (manual trigger)")
    assert_type_code(ev3, "PLATE_EVENT", "Device 3 (manual trigger)")


# ─── Test 2.5 – Raw JSON body ──────────────────────────────────────────────────
def test_raw_json_body(hdrs, dev1_sid, dev1_key):
    step("Test 2.5  ·  Raw JSON Body  (Device 1 — application/json)")

    before = count_events(hdrs, dev1_sid)
    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        json={"plate": "BB5678CC", "confidence": 0.981, "region": "UA"},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected JSON body ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted JSON body")

    ev = wait_for_new_event(hdrs, dev1_sid, before, "Device 1 JSON body")
    assert_raw_data_key(ev, "Device 1 JSON body", hdrs, expected_fragment="BB5678CC")
    assert_gate_id(ev, GATE_1, "Device 1 JSON body")
    assert_type_code(ev, "PLATE_EVENT", "Device 1 JSON body")


# ─── Test 2.6 – Form fields (Device 4, WEIGHT_EVENT) ─────────────────────────
def test_form_fields(hdrs, dev4_sid, dev4_key):
    step("Test 2.6  ·  Form Fields  (Device 4 — multipart, weight_kg field)")

    before = count_events(hdrs, dev4_sid)
    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev4_key},
        data={"weight_kg": "25400.0"},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected form-fields event ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted form fields")

    ev = wait_for_new_event(hdrs, dev4_sid, before, "Device 4 form fields")
    assert_raw_data_key(ev, "Device 4 form fields", hdrs, expected_fragment="weight_kg")
    assert_gate_id(ev, GATE_2, "Device 4 form fields")
    assert_type_code(ev, "WEIGHT_EVENT", "Device 4 form fields")


# ─── Test 2.7 – Raw XML body (Device 5, WEIGHT_EVENT) ────────────────────────
def test_raw_xml_body(hdrs, dev5_sid, dev5_key):
    step("Test 2.7  ·  Raw XML Body  (Device 5 — application/xml)")

    before = count_events(hdrs, dev5_sid)
    xml_body = "<?xml version='1.0' encoding='UTF-8'?><event><weight_kg>25400.0</weight_kg></event>"
    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev5_key, "Content-Type": "application/xml"},
        data=xml_body.encode("utf-8"),
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected XML body ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted XML body")

    ev = wait_for_new_event(hdrs, dev5_sid, before, "Device 5 XML body")
    assert_raw_data_key(ev, "Device 5 XML body", hdrs, expected_fragment="25400")
    assert_gate_id(ev, GATE_2, "Device 5 XML body")
    assert_type_code(ev, "WEIGHT_EVENT", "Device 5 XML body")


# ─── Test 2.8 – Transaction isolation ────────────────────────────────────────
def test_transaction_isolation(hdrs, dev1_sid, dev4_sid):
    step("Test 2.8  ·  Transaction Isolation  (GATE_1 vs GATE_2)")

    ev1 = get_latest_event(hdrs, dev1_sid)
    ev4 = get_latest_event(hdrs, dev4_sid)
    if not ev1: fail("No GATE_1 events — run tests 2.1–2.5 first")
    if not ev4: fail("No GATE_2 events — run tests 2.6–2.7 first")

    tx1, tx2 = ev1.get("transaction_id"), ev4.get("transaction_id")
    if not tx1: fail(f"GATE_1 event has no transaction_id")
    if not tx2: fail(f"GATE_2 event has no transaction_id")
    if tx1 == tx2:
        fail(f"Isolation violated: both gates share transaction_id={tx1}")
    ok(f"Transactions are isolated")
    info(f"GATE_1 tx={tx1[:8]}…  GATE_2 tx={tx2[:8]}…")


# ─── Test 2.9 – Matchmaker: external transaction_id ──────────────────────────
def test_matchmaker_external_transaction(hdrs, dev1_sid, dev1_key):
    step("Test 2.9  ·  Matchmaker — external transaction_id")

    ev_existing = get_latest_event(hdrs, dev1_sid)
    if not ev_existing: fail("No GATE_1 events — run test 2.1 first")
    tx_id = ev_existing.get("transaction_id")
    if not tx_id: fail("Latest GATE_1 event has no transaction_id")

    r_tx = requests.get(f"{BASE_URL}/api/v1/transactions/{tx_id}", headers=hdrs, timeout=10)
    if r_tx.status_code != 200:
        fail(f"Cannot fetch transaction {tx_id}: {r_tx.text}")
    events_before = len(r_tx.json().get("events") or [])

    # Part A: valid external transaction_id → must attach
    before_a = count_events(hdrs, dev1_sid)
    r_a = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        data={"payload": json.dumps({"plate": "MATCHMAKER-A"}), "transaction_id": tx_id},
        timeout=10,
    )
    if r_a.status_code != 202:
        fail(f"Ingestor rejected Part A ({r_a.status_code}): {r_a.text}")
    ev_a = wait_for_new_event(hdrs, dev1_sid, before_a, "Matchmaker Part A", timeout=25)
    if str(ev_a.get("transaction_id")) != str(tx_id):
        fail(f"Valid tx_id ignored — expected {tx_id}, got {ev_a.get('transaction_id')}")
    ok(f"Valid tx_id honoured  (tx={tx_id[:8]}…)")
    assert_type_code(ev_a, "PLATE_EVENT", "Matchmaker Part A")
    assert_raw_data_key(ev_a, "Matchmaker Part A", hdrs)

    r_tx2 = requests.get(f"{BASE_URL}/api/v1/transactions/{tx_id}", headers=hdrs, timeout=10)
    events_after = len(r_tx2.json().get("events") or [])
    if events_after <= events_before:
        fail(f"Event NOT added to target tx — before={events_before}, after={events_after}")
    ok(f"Event count: {events_before} → {events_after}")

    # Part B: non-existent transaction_id → must create new
    nil_tx = "00000000-0000-0000-0000-000000000000"
    before_b = count_events(hdrs, dev1_sid)
    r_b = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": dev1_key},
        data={"payload": json.dumps({"plate": "MATCHMAKER-B"}), "transaction_id": nil_tx},
        timeout=10,
    )
    if r_b.status_code != 202:
        fail(f"Ingestor rejected Part B ({r_b.status_code}): {r_b.text}")
    ev_b = wait_for_new_event(hdrs, dev1_sid, before_b, "Matchmaker Part B", timeout=25)
    if str(ev_b.get("transaction_id")) == nil_tx:
        fail("Nil tx_id was accepted — must fall back to a new transaction")
    ok(f"Nil tx_id rejected → new tx={str(ev_b.get('transaction_id'))[:8]}…")
    assert_type_code(ev_b, "PLATE_EVENT", "Matchmaker Part B")
    assert_raw_data_key(ev_b, "Matchmaker Part B", hdrs)

    # Part C: wrong-gate transaction_id → must create new
    r_txs = requests.get(f"{BASE_URL}/api/v1/transactions?gate_id={GATE_2}&limit=1",
                          headers=hdrs, timeout=10)
    if r_txs.status_code == 200:
        items = r_txs.json().get("data") or []
        if items:
            wrong_tx = items[0].get("id") or items[0].get("transaction_id")
            if wrong_tx:
                before_c = count_events(hdrs, dev1_sid)
                r_c = requests.post(
                    f"{BASE_URL}/ingest/event",
                    headers={"X-API-Key": dev1_key},
                    data={"payload": json.dumps({"plate": "MATCHMAKER-C"}),
                          "transaction_id": wrong_tx},
                    timeout=10,
                )
                if r_c.status_code != 202:
                    fail(f"Ingestor rejected Part C ({r_c.status_code}): {r_c.text}")
                ev_c = wait_for_new_event(hdrs, dev1_sid, before_c, "Matchmaker Part C", timeout=25)
                if str(ev_c.get("transaction_id")) == str(wrong_tx):
                    fail(f"Cross-gate tx accepted — GATE_2 tx used for GATE_1 event")
                ok(f"Cross-gate tx rejected → new tx={str(ev_c.get('transaction_id'))[:8]}…")
                assert_type_code(ev_c, "PLATE_EVENT", "Matchmaker Part C")
                assert_raw_data_key(ev_c, "Matchmaker Part C", hdrs)
                return
    info("Part C skipped — no GATE_2 transactions yet (run tests 2.6–2.7 first)")


# ─── History / fuzzy-search setup ─────────────────────────────────────────────
def setup_history_env(rv, hdrs):
    """
    Reuses PLATE_EVENT type (already created in setup()).
    Creates GATE_HISTORY with max_events_per_transaction=1 so transactions
    auto-close after each event (needed for history search).
    """
    step("History Test Environment")

    # Gate
    get_or_create_gate(hdrs, GATE_HISTORY, "History Test Gate",
                       {"transaction_ttl_minutes": 1, "max_events_per_transaction": 1})

    # Reuse PLATE_EVENT (created in setup)
    plate_type_id = get_or_create_event_type(
        rv, hdrs,
        code="PLATE_EVENT", name="Plate Event",
        fields={
            "plate":        {"type": "string", "required": True},
            "confidence":   {"type": "number", "required": False},
            "region":       {"type": "string", "required": False},
            "vehicle_type": {"type": "string", "required": False},
        },
        cache_key=K["type_plate"],
        searchable_key="plate",
    )

    hist_dev_sid, hist_dev_key = get_or_create_api_key(
        rv, hdrs, K["hist_dev_sid"], K["hist_dev_key"],
        name="Test – History Device", gate_id=GATE_HISTORY, perms=["ingest:events"],
    )
    ensure_device_config(
        hdrs, hist_dev_sid, plate_type_id,
        gate_id=GATE_HISTORY, data_type="json",
        data_mapping={"plate": "$.plate"},
        trigger_enabled=False, triggers=[],
    )
    return plate_type_id, hist_dev_sid, hist_dev_key


def _ingest_plate_event(hdrs, hist_dev_key, hist_dev_sid, plate, label="plate event"):
    before = count_events(hdrs, hist_dev_sid)
    r = requests.post(
        f"{BASE_URL}/ingest/event",
        headers={"X-API-Key": hist_dev_key},
        json={"plate": plate},
        timeout=10,
    )
    if r.status_code != 202:
        fail(f"Ingestor rejected plate event ({r.status_code}): {r.text}")
    return wait_for_new_event(hdrs, hist_dev_sid, before, label)


def _close_history_gate_transaction(rv):
    key = f"tx_active:{GATE_HISTORY}"
    rv.delete(key)
    info(f"Deleted Valkey key '{key}' — transaction now closed")


# ─── Test 2.10 – BeforeSave hook ──────────────────────────────────────────────
def test_searchable_value_hook(hdrs, plate_type_id, hist_dev_sid, hist_dev_key):
    step("Test 2.10  ·  BeforeSave Hook — searchable_value + type_code")

    ev = _ingest_plate_event(hdrs, hist_dev_key, hist_dev_sid, PLATE_EXACT, "BeforeSave probe")
    assert_type_code(ev, "PLATE_EVENT", "BeforeSave probe")
    assert_searchable_value(ev, PLATE_EXACT, "BeforeSave probe")
    info(f"event id={ev.get('id')}  tx={str(ev.get('transaction_id', ''))[:8]}…")


# ─── Test 2.11 / 2.12 – Vehicle history search ────────────────────────────────
def test_vehicle_history_search(hdrs, rv, hist_dev_sid, hist_dev_key):
    step("Test 2.11/2.12  ·  Vehicle History Search  (exact + fuzzy + no-match)")

    _ingest_plate_event(hdrs, hist_dev_key, hist_dev_sid, PLATE_EXACT, "History seed")
    _close_history_gate_transaction(rv)

    # Part A: exact match
    r_exact = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_EXACT}",
        headers=hdrs, timeout=10,
    )
    if r_exact.status_code != 200:
        fail(f"History exact ({r_exact.status_code}): {r_exact.text}")
    found = r_exact.json().get("data") or []
    if not found:
        fail(f"Exact search returned 0 results for '{PLATE_EXACT}'")
    ok(f"Exact match: {len(found)} transaction(s) for '{PLATE_EXACT}'")
    if any(len(tx.get("events") or []) > 0 for tx in found):
        ok("Transactions include preloaded events")
    else:
        warn("Transactions returned without events")

    # Part B: fuzzy match (distance=1)
    r_fuzzy = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_FUZZY}",
        headers=hdrs, timeout=10,
    )
    if r_fuzzy.status_code != 200:
        fail(f"History fuzzy ({r_fuzzy.status_code}): {r_fuzzy.text}")
    found_fuzzy = r_fuzzy.json().get("data") or []
    if not found_fuzzy:
        fail(f"Fuzzy search returned 0 results for '{PLATE_FUZZY}' (expected distance=1 match)")
    ok(f"Fuzzy match: {len(found_fuzzy)} transaction(s) for '{PLATE_FUZZY}' → '{PLATE_EXACT}'")

    # Part C: guaranteed no-match
    r_miss = requests.get(
        f"{BASE_URL}/api/v1/transactions/history?plate={PLATE_MISS}",
        headers=hdrs, timeout=10,
    )
    if r_miss.status_code != 200:
        fail(f"History no-match ({r_miss.status_code}): {r_miss.text}")
    found_miss = r_miss.json().get("data") or []
    if found_miss:
        warn(f"No-match '{PLATE_MISS}' returned {len(found_miss)} result(s)")
    else:
        ok(f"No-match '{PLATE_MISS}' correctly returns 0 results")


# ─── Test 2.13 – searchable_key CRUD ─────────────────────────────────────────
def test_searchable_key_crud(hdrs, plate_type_id, hist_dev_sid, hist_dev_key):
    step("Test 2.13  ·  searchable_key — GET returns it, PUT clears/restores")

    # Part A: verify GET /types returns searchable_key = "plate"
    all_types = requests.get(f"{BASE_URL}/api/v1/types", headers=hdrs, timeout=10).json()
    pt = next((t for t in all_types if t["id"] == plate_type_id), None)
    if pt is None:
        fail(f"PLATE_EVENT (id={plate_type_id}) not found in GET /types")
    if pt.get("searchable_key") != "plate":
        fail(f"searchable_key — expected 'plate', got '{pt.get('searchable_key')}'")
    ok(f"GET /types: searchable_key = 'plate'")

    # Part B: clear searchable_key → hook must not populate searchable_value
    r_clear = requests.put(
        f"{BASE_URL}/api/v1/types/{plate_type_id}",
        headers=hdrs, json={"searchable_key": ""}, timeout=10,
    )
    if r_clear.status_code not in (200, 204):
        fail(f"PUT clear searchable_key ({r_clear.status_code}): {r_clear.text}")
    ok("searchable_key cleared")

    ev_probe = _ingest_plate_event(hdrs, hist_dev_key, hist_dev_sid, PLATE_EXACT, "sk probe")
    sv = ev_probe.get("searchable_value", "")
    if sv:
        fail(f"searchable_value should be empty after clear, got '{sv}'")
    ok("searchable_value empty (hook respects cleared config)")

    # Part C: restore
    r_restore = requests.put(
        f"{BASE_URL}/api/v1/types/{plate_type_id}",
        headers=hdrs, json={"searchable_key": "plate"}, timeout=10,
    )
    if r_restore.status_code not in (200, 204):
        fail(f"PUT restore searchable_key ({r_restore.status_code}): {r_restore.text}")
    ok("searchable_key restored to 'plate'")


# ─── Test 2.14 – ITSAPI camera: Digest Auth + Base64 image ───────────────────
def test_itsapi_digest_ingest(hdrs, itsapi_dev_sid):
    """
    Simulates an ITSAPI-protocol ANPR camera:

    Authentication flow (handled transparently by HTTPDigestAuth):
      1. requests sends the POST without Authorization.
      2. NGINX forwards to /auth_validate → Auth service returns 401 +
         WWW-Authenticate: Digest realm="omnigate", nonce="<fresh>".
      3. NGINX captures the header and returns it to the client.
      4. requests.HTTPDigestAuth computes the response hash and retries
         with Authorization: Digest username=..., response=...
      5. Auth service validates the hash, injects X-Source-ID / X-Gate-ID /
         X-Permissions, and NGINX proxies the request to Ingestor.

    Image handling (on the Adapter side):
      - The Adapter reads config["image_fields"] = ["front_image"].
      - It base64-decodes the value at $.imageData, uploads the bytes to
        Garage/S3, and replaces the field with the resulting object key.
      - Core stores the event with the S3 key instead of the raw blob.
    """
    step("Test 2.14  ·  ITSAPI Camera — HTTP Digest Auth + Base64 Image in JSON")

    before = count_events(hdrs, itsapi_dev_sid)

    # Encode the test image to a plain base64 string (no data-URI prefix).
    image_b64 = encode_image_base64(ITSAPI_IMAGE)
    info(f"Base64 payload size: {len(image_b64)} chars")

    # Build the ITSAPI JSON payload.
    # Field names match the data_mapping in Device 6's DeviceConfig:
    #   "plate"       ← $.lp
    #   "confidence"  ← $.confidence
    #   "front_image" ← $.imageData
    payload = {
        "lp":         "UA1234BB",
        "confidence": 0.987,
        "imageData":  image_b64,
        "timestamp":  time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "device_id":  ITSAPI_USER,
        "speed_kmh":  65.3,
    }

    try:
        r = requests.post(
            f"{BASE_URL}/ingest/event",
            # HTTPDigestAuth intercepts the 401 challenge and automatically
            # retries with the correct Authorization: Digest ... header.
            auth=HTTPDigestAuth(ITSAPI_USER, ITSAPI_PASSWORD),
            # json= serialises to JSON and sets Content-Type: application/json.
            # We override it to match ITSAPI spec (charset annotation).
            json=payload,
            headers={"Content-Type": "application/json;charset=UTF-8"},
            timeout=15,
        )
    except requests.exceptions.ConnectionError as exc:
        fail(f"ITSAPI connection error: {exc}")

    info(f"Response: HTTP {r.status_code}  body={r.text[:120]}")

    if r.status_code != 202:
        fail(f"Ingestor rejected ITSAPI event ({r.status_code}): {r.text}")
    ok(f"Ingestor accepted ITSAPI event  (transaction_id={r.json().get('transaction_id')})")

    # Wait for the Adapter to process the event and Core to persist it.
    ev = wait_for_new_event(hdrs, itsapi_dev_sid, before, "ITSAPI camera", timeout=30)
    assert_gate_id(ev, GATE_1, "ITSAPI camera")
    assert_type_code(ev, "PLATE_EVENT", "ITSAPI camera")
    assert_raw_data_key(ev, "ITSAPI camera", hdrs)

    # The plate number comes from JSONPath $.lp → mapped to "plate".
    data = ev.get("data") or {}
    if data.get("plate") == "UA1234BB":
        ok(f"plate correctly mapped from $.lp  ('{data['plate']}')")
    else:
        warn(f"plate field unexpected: {data.get('plate')!r}")

    # Verify that the Adapter replaced the raw base64 blob with an S3 object key.
    # The Adapter uploads to itsapi/{source_id}/YYYY/MM/DD/{uuid}_front_image.jpg
    # and stores that path in place of the original base64 string.
    front_image = data.get("front_image", "")
    if front_image.startswith("itsapi/"):
        ok(f"Base64 blob replaced by S3 key: {front_image}")
    elif front_image:
        warn(
            f"front_image present but not an S3 key (Adapter may still be processing): "
            f"{front_image[:80]}…"
        )
    else:
        warn("front_image field absent from Core event — check Adapter logs")


# ─── Gate helpers for load test ───────────────────────────────────────────────
def get_or_create_gate(hdrs, gate_id, name, settings):
    all_gates = requests.get(f"{BASE_URL}/api/v1/gates", headers=hdrs, timeout=10).json()
    existing = next((g for g in all_gates if g.get("gate_id") == gate_id), None)
    if existing:
        requests.put(
            f"{BASE_URL}/api/v1/gates/{existing['id']}", headers=hdrs,
            json={"gate_id": gate_id, "name": name, "settings": settings}, timeout=10,
        )
        skip(f"Gate '{gate_id}'  (id={existing['id']})")
        return existing["id"]
    r = requests.post(
        f"{BASE_URL}/api/v1/gates", headers=hdrs,
        json={"gate_id": gate_id, "name": name, "settings": settings}, timeout=10,
    )
    if r.status_code != 201:
        fail(f"Cannot create gate '{gate_id}': {r.text}")
    gate_uuid = r.json().get("id")
    ok(f"Created gate '{gate_id}'  (id={gate_uuid})")
    return gate_uuid


# ─── Load test ────────────────────────────────────────────────────────────────
def run_much(rv, hdrs, num_events):
    step(f"Load Test  ·  {num_events} events")
    GATE_MUCH = "gate-much"
    get_or_create_gate(hdrs, GATE_MUCH, "Load Test Gate",
                       {"transaction_ttl_minutes": 1, "max_events_per_transaction": 5})

    plate_type_id = get_or_create_event_type(
        rv, hdrs, code="PLATE_MUCH", name="Plate (Much)",
        fields={"plate": {"type": "string", "required": True}},
        cache_key="omnigate:test:type_plate_much",
    )
    weight_type_id = get_or_create_event_type(
        rv, hdrs, code="WEIGHT_MUCH", name="Weight (Much)",
        fields={"weight_kg": {"type": "number", "required": True}},
        cache_key="omnigate:test:type_weight_much",
    )

    puller_sid, _ = get_or_create_api_key(
        rv, hdrs, "omnigate:test:much_puller_sid", "omnigate:test:much_puller_key",
        name="Load Puller Target", gate_id=GATE_MUCH, perms=["ingest:events"],
    )
    ensure_device_config(
        hdrs, puller_sid, plate_type_id, gate_id=GATE_MUCH,
        data_type="json", data_mapping={"plate": "$.plate"},
        trigger_enabled=False, triggers=[], trigger_url=TRIGGER_URL,
    )

    devs = []
    for i in range(5):
        is_cam = (i % 2 == 0)
        sid, key = get_or_create_api_key(
            rv, hdrs, f"omnigate:test:much_dev{i}_sid", f"omnigate:test:much_dev{i}_key",
            name=f"Load Dev {i}", gate_id=GATE_MUCH, perms=["ingest:events"],
        )
        ensure_device_config(
            hdrs, sid, plate_type_id if is_cam else weight_type_id,
            gate_id=GATE_MUCH, data_type="json",
            data_mapping={"plate": "$.plate"} if is_cam else {"weight_kg": "$.weight_kg"},
            trigger_enabled=not is_cam,
            triggers=[{"source_id": puller_sid}] if not is_cam else [],
        )
        devs.append((sid, key, is_cam))

    info(f"Sending {num_events} events …")
    start_t = time.time()
    for n in range(1, num_events + 1):
        sid, key, is_cam = random.choice(devs)
        payload = ({"plate": f"AA{random.randint(1000,9999)}BB"} if is_cam
                   else {"weight_kg": random.randint(1000, 40000)})
        r = requests.post(f"{BASE_URL}/ingest/event", headers={"X-API-Key": key},
                          json=payload, timeout=5)
        if r.status_code != 202:
            warn(f"Event {n} failed: {r.status_code} {r.text}")
        elif n % 50 == 0 or n == num_events:
            print(f"  ... {n}/{num_events}", end="\r")
    dur = time.time() - start_t
    print()
    ok(f"Generated {num_events} events in {dur:.2f}s ({num_events/dur:.1f} ev/s)")


# ─── Main ─────────────────────────────────────────────────────────────────────
def main():
    if not ADMIN_PASS:
        fail("ADMIN_DEFAULT_PASSWORD env var is not set")

    parser = argparse.ArgumentParser(description="OmniGate integration test")
    parser.add_argument("--reset", action="store_true",
                        help="Delete all test resources from system + Valkey, then recreate")
    parser.add_argument("--much", type=int, metavar="N",
                        help="Stress test: generate N events across multiple devices")
    args = parser.parse_args()

    try:
        rv = Redis.from_url(VALKEY_URL, decode_responses=True, socket_connect_timeout=3)
        rv.ping()
    except Exception as e:
        fail(f"Cannot connect to Valkey at {VALKEY_URL}: {e}")

    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}  OmniGate Integration Test{RESET}")
    print(f"  Gateway : {BASE_URL}")
    print(f"  Valkey  : {VALKEY_URL}")
    print(f"{BOLD}{'=' * 60}{RESET}")

    step("Login")
    hdrs = login()
    ok("Logged in as admin")

    if args.reset:
        reset_all(rv, hdrs)

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
        plate_type_id,
    ) = setup(rv, hdrs)

    test_camera_ingest(hdrs, dev1_sid, dev1_key)
    test_puller_config_resolution(hdrs, dev2_sid, dev3_sid)
    test_scale_with_trigger(hdrs, dev2_sid, dev2_key, dev3_sid)
    test_manual_trigger(hdrs, dev2_cfg_id, dev3_sid)
    test_raw_json_body(hdrs, dev1_sid, dev1_key)
    test_form_fields(hdrs, dev4_sid, dev4_key)
    test_raw_xml_body(hdrs, dev5_sid, dev5_key)
    test_transaction_isolation(hdrs, dev1_sid, dev4_sid)
    test_matchmaker_external_transaction(hdrs, dev1_sid, dev1_key)

    # ── ITSAPI / Digest Auth tests ──────────────────────────────────────────
    # Device 6 reuses the PLATE_EVENT type already created in setup().
    itsapi_dev_sid = setup_itsapi_device(rv, hdrs, plate_type_id)
    test_itsapi_digest_ingest(hdrs, itsapi_dev_sid)

    plate_type_id, hist_dev_sid, hist_dev_key = setup_history_env(rv, hdrs)
    test_searchable_value_hook(hdrs, plate_type_id, hist_dev_sid, hist_dev_key)
    test_vehicle_history_search(hdrs, rv, hist_dev_sid, hist_dev_key)
    test_searchable_key_crud(hdrs, plate_type_id, hist_dev_sid, hist_dev_key)

    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}{GREEN}  All tests passed ✓{RESET}")
    print(f"{BOLD}{'=' * 60}{RESET}\n")


if __name__ == "__main__":
    main()
