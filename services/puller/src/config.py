import os
from dataclasses import dataclass

@dataclass(frozen=True)
class Config:
    INGESTOR_URL: str = os.getenv("INGESTOR_URL", "http://gateway/ingest")
    WORKER_API_KEY: str = os.getenv("WORKER_API_KEY", "")
    REDIS_ADDR: str = os.getenv("VALKEY_ADDR", "valkey:6379")
    STREAM_PULLER: str = "events:puller"
    STREAM_DLQ: str = "events:dlq"

cfg = Config()
