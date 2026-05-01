import json
import logging
from typing import Dict, Optional
from redis import Redis
from src.config import cfg

logger = logging.getLogger(__name__)


class PullerClient:
    def __init__(self, redis: Redis):
        self._redis = redis

    def trigger_pull(
        self,
        trigger_url: str,
        transaction_id: str,
        gate_id: str,
        source_id: str,
        event_data: Dict,
        trigger_source_id: Optional[str] = None,
    ) -> None:
        msg: Dict = {
            "trigger_url": trigger_url,
            "transaction_id": transaction_id,
            "gate_id": gate_id,
            "source_id": source_id,
            "context": event_data,
        }
        if trigger_source_id:
            msg["trigger_source_id"] = trigger_source_id

        self._redis.xadd(cfg.STREAM_PULLER, {"data": json.dumps(msg)})
        logger.info(f"Queued pull task for tx {transaction_id} → {cfg.STREAM_PULLER}")
