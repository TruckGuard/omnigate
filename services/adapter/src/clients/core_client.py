import requests
from typing import Dict, Optional
from src.config import cfg
import logging

logger = logging.getLogger(__name__)

class CoreClient:
    def __init__(self):
        self.base_url = cfg.CORE_URL
        self.headers = {
            "X-API-Key": cfg.WORKER_SYSTEM_KEY
        }
        
    def get_device_config(self, source_id: str) -> Optional[Dict]:
        """Fetch device configuration from CORE service."""
        url = f"{self.base_url}/configs/devices/{source_id}"
        
        try:
            response = requests.get(url, headers=self.headers, timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            logger.error(f"Failed to fetch device config for {source_id}: {e}")
            return None
    
    def create_event(
        self,
        event_type_id: str,
        gate_id: str,
        source_id: str,
        data: Dict,
        raw_data_key: str,
        image_keys: list = None,
        transaction_id: Optional[str] = None,
        ingested_at: Optional[str] = None,
    ) -> Dict:
        """Create an event in CORE service."""
        url = f"{self.base_url}/events"

        payload = {
            "event_type_id": event_type_id,
            "gate_id": gate_id,
            "source_id": source_id,
            "data": data,
            "raw_data_key": raw_data_key,
            "image_keys": image_keys or [],
        }

        if transaction_id:
            payload["transaction_id"] = transaction_id
        if ingested_at:
            payload["ingested_at"] = ingested_at
        
        try:
            response = requests.post(url, json=payload, headers=self.headers, timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            logger.error(f"Failed to create event: {e}")
            raise