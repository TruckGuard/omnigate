from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Dict, Optional
import requests
import logging
from src.clients.ingestor_client import IngestorClient
from src.config import cfg

logger = logging.getLogger(__name__)
router = APIRouter()
ingestor = IngestorClient(cfg.INGESTOR_URL)


class PullRequest(BaseModel):
    trigger_url: str
    transaction_id: str
    gate_id: str
    source_id: str
    trigger_source_id: Optional[str] = None  # Source ID to assume when sending to Ingestor
    payload: Optional[Dict] = None  # Optional payload to send with the pulled data
    context: Optional[Dict] = None


@router.post("/pull")
async def pull_data(req: PullRequest):
    """
    Fetch data from external URL and send back to Ingestor.
    """
    logger.info(f"Pulling data from {req.trigger_url} for transaction {req.transaction_id}")
    
    try:
        # 1. Fetch data from trigger_url
        response = requests.get(req.trigger_url, timeout=15)
        response.raise_for_status()
        
        content_type = response.headers.get("Content-Type", "")
        logger.info(f"Fetched data from {req.trigger_url}, Content-Type: {content_type}")

        # 2. Prepare data for Ingestor
        endpoint = "event"
        files = None
        payload = req.payload or {}

        if content_type.startswith("image/"):
            logger.info("Fetched content is an image. Sending as file.")
            files = {
                "image": ("pulled_image.jpg", response.content, content_type)
            }
            # If no payload provided for image, set a default event type
            if not payload:
                payload = {
                    "event_type": "camera_recognition",
                    "plate": "PULLED_IMG",
                    "confidence": 1.0,
                    "direction": "unknown"
                }
        else:
            # Assume JSON
            try:
                fetched_data = response.json()
                logger.info(f"Fetched JSON data: {fetched_data}")
                # Merge or use fetched data as payload
                if not payload:
                    payload = fetched_data
                else:
                    payload.update(fetched_data)
            except Exception:
                logger.warning("Failed to parse fetched data as JSON. Sending as raw payload.")
                payload = {"raw_data": response.text}

        # 3. Send data back to Ingestor
        ingestor.send_data(
            endpoint=endpoint,
            payload=payload,
            transaction_id=req.transaction_id,
            files=files,
            assume_source_id=req.trigger_source_id,
        )
        
        return {
            "status": "success",
            "transaction_id": req.transaction_id,
            "content_type": content_type
        }
    
    except requests.RequestException as e:
        logger.error(f"Failed to pull data from {req.trigger_url}: {e}")
        raise HTTPException(status_code=502, detail=f"External request failed: {str(e)}")
    except Exception as e:
        logger.error(f"Error processing pull request: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/health")
async def health():
    return {"status": "ok"}
