import json
import requests
import logging
from src.clients.ingestor_client import IngestorClient
from src.clients.rtsp_client import grab_frame_jpeg
from src.config import cfg

logger = logging.getLogger(__name__)

_RTSP_SCHEMES = ("rtsp://", "rtsps://")


class PullWorker:
    def __init__(self):
        self.ingestor = IngestorClient(cfg.INGESTOR_URL)

    def _get_trigger_url(self, trigger_source_id: str) -> str:
        """Fetch the target device's config from Core and return its polling URL."""
        resp = requests.get(
            f"{cfg.CORE_URL}/configs/devices/{trigger_source_id}", timeout=10
        )
        resp.raise_for_status()
        device_config = resp.json()
        trigger_url = (device_config.get("trigger_url") or "").strip()
        if not trigger_url:
            raise ValueError(f"No trigger_url configured for device {trigger_source_id}")
        return trigger_url

    def _fetch_rtsp(self, trigger_url: str) -> tuple[dict, dict]:
        """Grab a single frame from an RTSP camera; return (payload, files)."""
        logger.info(f"Capturing RTSP frame from {trigger_url}")
        image_bytes = grab_frame_jpeg(trigger_url)
        files = {"image": ("frame.jpg", image_bytes, "image/jpeg")}
        return {}, files

    def _fetch_http(self, trigger_url: str, payload: dict) -> tuple[dict, dict | None]:
        """HTTP GET the trigger URL; return (merged_payload, files)."""
        response = requests.get(trigger_url, timeout=15)
        response.raise_for_status()

        content_type = response.headers.get("Content-Type", "")
        files = None

        if content_type.startswith("image/"):
            files = {"image": ("pulled_image.jpg", response.content, content_type)}
        else:
            try:
                fetched = response.json()
                payload = {**payload, **fetched} if payload else fetched
            except Exception:
                logger.warning("Fetched data is not JSON, using raw text")
                payload = {"raw_data": response.text}

        return payload, files

    def process(self, raw: str) -> None:
        msg = json.loads(raw)

        trigger_source_id = msg["trigger_source_id"]
        transaction_id    = msg["transaction_id"]
        gate_id           = msg["gate_id"]
        payload           = msg.get("context") or {}

        logger.info(f"Resolving trigger_url for {trigger_source_id}, tx {transaction_id}")

        trigger_url = self._get_trigger_url(trigger_source_id)

        logger.info(f"Pulling {trigger_url} for tx {transaction_id}")

        if trigger_url.lower().startswith(_RTSP_SCHEMES):
            payload, files = self._fetch_rtsp(trigger_url)
        else:
            payload, files = self._fetch_http(trigger_url, payload)

        self.ingestor.send_data(
            endpoint="event",
            payload=payload,
            transaction_id=transaction_id,
            files=files,
            assume_source_id=trigger_source_id,
            gate_id=gate_id,
        )

        logger.info(f"Pull complete for tx {transaction_id}")
