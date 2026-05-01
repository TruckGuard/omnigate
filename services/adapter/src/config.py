import os
from dataclasses import dataclass

@dataclass(frozen=True)
class Config:
    WORKER_SYSTEM_KEY: str = os.getenv("WORKER_SYSTEM_KEY", "")
    CORE_URL: str = os.getenv("CORE_URL", "http://gateway/api/v1")
    PULLER_URL: str = os.getenv("PULLER_URL", "http://puller:8000")
    ANPR_URL: str = os.getenv("ANPR_URL", "http://anpr:8000/recognize")
    
    REDIS_ADDR: str = os.getenv("VALKEY_ADDR", "valkey:6379")
    STREAM_RAW: str = "events:adapter"
    STREAM_DLQ: str = "events:dlq"
    
    STORAGE_ENDPOINT: str = os.getenv("STORAGE_ENDPOINT", "garage:3900")
    STORAGE_ACCESS_KEY: str = os.getenv("STORAGE_ACCESS_KEY", "")
    STORAGE_SECRET_KEY: str = os.getenv("STORAGE_SECRET_KEY", "")
    STORAGE_BUCKET: str = os.getenv("STORAGE_BUCKET", "truckguard-data")
    
    CACHE_TTL: int = 300  # 5 minutes

cfg = Config()
