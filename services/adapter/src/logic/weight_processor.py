import json
from src.utils.logging_utils import logger

class WeightProcessor:
    def __init__(self, core_client, parser):
        self.core = core_client
        self.parser = parser
        self.config_cache = {}

    def _get_cached_config(self, source_id: str):
        if source_id in self.config_cache:
            return self.config_cache[source_id]
        config = self.core.get_scale_config(source_id)
        if config:
            self.config_cache[source_id] = config
        return config

    def process(self, data: dict):
        source_id = data.get("source_id")

        config = self._get_cached_config(source_id)
        logger.debug("Scale config retrieved", extra={"source_id": source_id, "config": config})
        if not config:
            logger.error("Config not found for scale", extra={"source_id": source_id})
            return

        mapping = config.get("field_mapping", {})
        if isinstance(mapping, str):
            mapping = json.loads(mapping)

        weight = self.parser.extract_value(
            data.get("payload"), config.get("format"), mapping
        )

        if weight is not None:
            final_event = {
                "scale_source_id": source_id,
                "scale_id": source_id,
                "weight": weight,
                "timestamp": data.get("at"),
                "raw_payload": data.get("payload"),
            }
            try:
                self.core.send_weight_event(final_event)
                logger.info("Processed weight successfully", extra={
                    "source_id": source_id,
                    "weight": weight,
                    "scale_id": final_event["scale_id"]
                })
            except Exception as e:
                logger.error("Failed to send weight event to Core", extra={"error": str(e)})
                raise e
        else:
            logger.warning("Could not extract weight", extra={"source_id": source_id})
