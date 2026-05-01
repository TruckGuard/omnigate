import os
from dataclasses import dataclass

@dataclass(frozen=True)
class Config:
    INGESTOR_URL: str = os.getenv("INGESTOR_URL", "http://gateway/ingest")
    PORT: int = int(os.getenv("PORT", "8000"))
    WORKER_API_KEY: str = os.getenv("WORKER_API_KEY", "")

cfg = Config()
