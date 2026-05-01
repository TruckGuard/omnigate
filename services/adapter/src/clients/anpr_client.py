import requests
from typing import List, Dict, Any
from src.config import cfg
from src.utils.logging_utils import logger
from tenacity import retry, stop_after_attempt, wait_exponential

class ANPRClient:
    def __init__(self):
        self.url = cfg.ANPR_URL

    @retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=1, max=5))
    def recognize(self, image_bytes: bytes) -> List[Dict[str, Any]]:
        """Повертає список варіантів номерів із впевненістю"""
        try:
            files = {"file": ("image.jpg", image_bytes, "image/jpeg")}
            resp = requests.post(self.url, files=files, timeout=15)
            
            if resp.status_code == 200:
                data = resp.json()
                logger.info("ANPR response received", extra={"plates_count": len(data.get("plates", []))})
                return data.get("plates", [])
            return []
        except Exception as e:
            logger.error("ANPR Request failed", extra={"error": str(e)})
            return []