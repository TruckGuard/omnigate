import requests
from typing import Dict, Optional
from src.config import cfg
import logging

logger = logging.getLogger(__name__)

class PullerClient:
    def __init__(self):
        self.base_url = cfg.PULLER_URL
    
    def trigger_pull(
        self,
        trigger_url: str,
        transaction_id: str,
        gate_id: str,
        source_id: str,
        event_data: Dict,
        trigger_source_id: Optional[str] = None,
    ) -> None:
        """Trigger PULLER to fetch external data."""
        url = f"{self.base_url}/pull"
        
        payload = {
            "trigger_url": trigger_url,
            "transaction_id": transaction_id,
            "gate_id": gate_id,
            "source_id": source_id,
            "context": event_data,  # Pass event data as context
        }

        if trigger_source_id:
            payload["trigger_source_id"] = trigger_source_id
            logger.info(f"Triggering puller with source assumption: {trigger_source_id}")
        
        try:
            response = requests.post(url, json=payload, timeout=5)
            response.raise_for_status()
            logger.info(f"Triggered puller for transaction {transaction_id}")
        except requests.RequestException as e:
            logger.error(f"Failed to trigger puller: {e}")
            # Don't raise - puller is optional

