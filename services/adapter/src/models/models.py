from dataclasses import dataclass, field
from typing import Dict, Any, Optional

@dataclass
class CameraConfig:
    id: str
    name: str
    format: str
    run_anpr: bool
    field_mapping: Dict[str, str] = field(default_factory=dict)

@dataclass
class IncomingEvent:
    source_id: str
    image_key: str
    payload: str
    timestamp: str