import json
import requests
import logging
from typing import Optional
from src.config import cfg

logger = logging.getLogger(__name__)


class IngestorClient:
    def __init__(self, base_url: str):
        self.base_url = base_url

    def send_data(
        self,
        endpoint: str,
        payload: dict,
        transaction_id: str,
        files: dict = None,
        assume_source_id: Optional[str] = None,
        gate_id: Optional[str] = None,
    ) -> dict:
        """Send fetched data back to Ingestor.

        When *files* is provided the request must be multipart so the binary
        content can be included.  Without files a plain JSON body is used; the
        Ingestor reads the raw body and extracts the Puller envelope fields
        (source_id, transaction_id, payload).
        """
        url = f"{self.base_url}/{endpoint}"
        headers = {"X-API-Key": cfg.WORKER_API_KEY}

        try:
            if files:
                # Binary content present — must use multipart/form-data.
                data = {
                    "payload": json.dumps(payload),
                    "transaction_id": transaction_id,
                }
                if assume_source_id:
                    data["source_id"] = assume_source_id
                if gate_id:
                    data["gate_id"] = gate_id
                if assume_source_id or gate_id:
                    logger.info(
                        "Assuming source identity (multipart)",
                        extra={"assume_source_id": assume_source_id, "gate_id": gate_id, "transaction_id": transaction_id},
                    )
                response = requests.post(url, data=data, files=files, headers=headers, timeout=15)
            else:
                # No binary content — send a JSON envelope that Ingestor can parse.
                body: dict = {
                    "payload": payload,
                    "transaction_id": transaction_id,
                }
                if assume_source_id:
                    body["source_id"] = assume_source_id
                if gate_id:
                    body["gate_id"] = gate_id
                if assume_source_id or gate_id:
                    logger.info(
                        "Assuming source identity (json)",
                        extra={"assume_source_id": assume_source_id, "gate_id": gate_id, "transaction_id": transaction_id},
                    )
                response = requests.post(url, json=body, headers=headers, timeout=10)

            response.raise_for_status()
            logger.info(f"Sent data to {url} for transaction {transaction_id}")
            return response.json()

        except requests.RequestException as e:
            logger.error(f"Failed to send data to Ingestor: {e}")
            if e.response is not None:
                logger.error(f"Response: {e.response.text}")
            raise
