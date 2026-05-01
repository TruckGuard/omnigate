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
    ):
        """Send fetched data back to Ingestor."""
        url = f"{self.base_url}/{endpoint}"
        
        # Add transaction_id to link this data to existing transaction
        import json
        
        data = {
            "payload": json.dumps(payload),
            "transaction_id": transaction_id,
        }

        # If trigger_source_id is set, pass it so the Ingestor assumes that device's identity
        if assume_source_id:
            data["source_id"] = assume_source_id
            logger.info(f"Assuming source identity: {assume_source_id} for transaction {transaction_id}")
        
        headers = {
            "X-API-Key": cfg.WORKER_API_KEY
        }

        try:
            if files:
                response = requests.post(url, data=data, files=files, headers=headers, timeout=15)
            else:
                response = requests.post(url, data=data, headers=headers, timeout=10)
                
            response.raise_for_status()
            logger.info(f"Sent data to {url} for transaction {transaction_id}")
            return response.json()
        except requests.RequestException as e:
            logger.error(f"Failed to send data to Ingestor: {e}")
            if e.response:
                logger.error(f"Response: {e.response.text}")
            raise
