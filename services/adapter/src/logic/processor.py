import json
import logging
from typing import Dict, Any
import xmltodict
from jsonpath_ng import parse

from src.clients.core_client import CoreClient
from src.clients.puller_client import PullerClient
from src.clients.anpr_client import ANPRClient
from src.clients.minio_client import MinioStorage

logger = logging.getLogger(__name__)

class EventProcessor:
    def __init__(
        self,
        core: CoreClient,
        puller: PullerClient,
        storage: MinioStorage,
        anpr: ANPRClient
    ):
        self.core = core
        self.puller = puller
        self.storage = storage
        self.anpr = anpr
        self.config_cache: Dict[str, Dict] = {}
        self.event_type_cache: Dict[str, Dict] = {}
    
    def process(self, raw_event: str):
        """Process a raw event from Valkey Stream."""
        event = json.loads(raw_event)
        
        source_id = event["source_id"]
        gate_id = event["gate_id"]
        
        logger.info(f"Processing event from {source_id} at gate {gate_id}")
        
        # 1. Get device configuration (with caching)
        config = self._get_config(source_id)
        if not config:
            raise ValueError(f"No config found for source {source_id}")
        
        if not config.get("enabled"):
            logger.warning(f"Config for {source_id} is disabled, skipping")
            return
        
        # 2. Parse RAW payload
        payload = event.get("payload", "{}")
        parsed_data = self._parse_payload(payload, config.get("data_type", "json"))
        
        # 3. Transform data using mapping
        transformed_data = self._transform_data(parsed_data, config.get("data_mapping", {}))
        
        # 4. Special handling for camera images (trigger ANPR)
        if event.get("image_keys"):
            image_key = event["image_keys"][0]
            anpr_result = self.anpr.recognize(image_key)
            if anpr_result:
                # Merge ANPR results into transformed data
                transformed_data.update({
                    "plate": anpr_result.get("plate"),
                    "confidence": anpr_result.get("confidence"),
                })
        
        # Validate against schema
        self._validate_data(transformed_data, config)
        
        # 5. Create event in CORE
        response = self.core.create_event(
            event_type_id=config["event_type_id"],
            gate_id=gate_id,
            source_id=source_id,
            data=transformed_data,
            raw_data_key=event.get("raw_storage_key", ""),
            image_keys=event.get("image_keys", []),
            transaction_id=event.get("transaction_id"),  # From Puller flow
        )
        
        transaction_id = response.get("transaction_id")
        logger.info(f"Event created with transaction {transaction_id}")
        
        # 6. Trigger PULLER if configured
        if config.get("trigger_enabled") and config.get("trigger_url"):
            self.puller.trigger_pull(
                trigger_url=config["trigger_url"],
                transaction_id=transaction_id,
                gate_id=gate_id,
                source_id=source_id,
                event_data=transformed_data,
                trigger_source_id=config.get("trigger_source_id"),
            )
    
    def _get_config(self, source_id: str) -> Dict:
        """Get device config with caching."""
        if source_id not in self.config_cache:
            config = self.core.get_device_config(source_id)
            if config:
                self.config_cache[source_id] = config
        return self.config_cache.get(source_id)
    
    def _parse_payload(self, payload: str, data_type: str) -> Dict:
        """Parse payload based on data type."""
        if data_type == "xml":
            return xmltodict.parse(payload)
        else:
            return json.loads(payload)
    
    def _transform_data(self, raw_data: Dict, mapping: Dict) -> Dict:
        """Transform raw data using JSONPath mapping."""
        result = {}
        
        for field, path in mapping.items():
            try:
                jsonpath_expr = parse(path)
                matches = jsonpath_expr.find(raw_data)
                if matches:
                    result[field] = matches[0].value
                else:
                    logger.warning(f"No match for path {path}")
            except Exception as e:
                logger.error(f"Error applying path {path}: {e}")
        
        return result
    
    def _validate_data(self, data: Dict, config: Dict):
        """Validate mapped data against event type schema."""
        event_type = config.get("event_type", {})
        fields = event_type.get("fields", {})
        
        for field_name, field_def in fields.items():
            required = field_def.get("required", False)
            if required and field_name not in data:
                logger.warning(f"Missing required field {field_name} in mapped data")
                # Could raise error here if strict
