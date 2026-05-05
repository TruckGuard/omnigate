import json
import logging
from typing import Dict
from redis import Redis
from src.config import cfg

logger = logging.getLogger(__name__)


class PullerClient:
    def __init__(self, redis: Redis):
        self._redis = redis

    def trigger_pull(
        self,
        trigger_source_id: str,
        transaction_id: str,
        gate_id: str,
        source_id: str,
        event_data: Dict,
    ) -> None:
        msg: Dict = {
            "trigger_source_id": trigger_source_id,
            "transaction_id": transaction_id,
            "gate_id": gate_id,
            "source_id": source_id,
            "context": event_data,
        }

        # Inject Trace Context
        try:
            from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
            carrier = {}
            TraceContextTextMapPropagator().inject(carrier)
            if "traceparent" in carrier:
                msg["trace_context"] = carrier["traceparent"]
        except Exception:
            pass

        self._redis.xadd(cfg.STREAM_PULLER, {"data": json.dumps(msg)})
        logger.info(f"Queued pull task for tx {transaction_id} → {cfg.STREAM_PULLER}")
