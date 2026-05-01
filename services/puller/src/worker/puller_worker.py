import json
import requests
import logging
from src.clients.ingestor_client import IngestorClient
from src.config import cfg

logger = logging.getLogger(__name__)


class PullWorker:
    def __init__(self):
        self.ingestor = IngestorClient(cfg.INGESTOR_URL)

    def process(self, raw: str) -> None:
        msg = json.loads(raw)

        trigger_url     = msg["trigger_url"]
        transaction_id  = msg["transaction_id"]
        gate_id         = msg["gate_id"]
        trigger_source_id = msg.get("trigger_source_id")
        payload         = msg.get("context") or {}

        logger.info(f"Pulling {trigger_url} for tx {transaction_id}")

        response = requests.get(trigger_url, timeout=15)
        response.raise_for_status()

        content_type = response.headers.get("Content-Type", "")
        files = None

        if content_type.startswith("image/"):
            files = {"image": ("pulled_image.jpg", response.content, content_type)}
            if not payload:
                payload = {
                    "event_type": "camera_recognition",
                    "plate": "PULLED_IMG",
                    "confidence": 1.0,
                    "direction": "unknown",
                }
        else:
            try:
                fetched = response.json()
                payload = {**payload, **fetched} if payload else fetched
            except Exception:
                logger.warning("Fetched data is not JSON, using raw text")
                payload = {"raw_data": response.text}

        self.ingestor.send_data(
            endpoint="event",
            payload=payload,
            transaction_id=transaction_id,
            files=files,
            assume_source_id=trigger_source_id,
        )

        logger.info(f"Pull complete for tx {transaction_id}")
