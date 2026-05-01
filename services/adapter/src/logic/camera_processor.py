import json
import time
from src.utils.logging_utils import logger

# Camera configs are cached for 5 minutes. This covers rapid bursts from the same
# camera without hitting the core service on every event, while still picking up
# config changes made via the UI within a reasonable time.
_CONFIG_CACHE_TTL = 300  # seconds

class CameraProcessor:
    def __init__(self, core_client, parser, minio_client, anpr_client):
        self.core = core_client
        self.parser = parser
        self.minio = minio_client
        self.anpr = anpr_client
        # {source_id: (config, expires_at)}
        self._config_cache: dict = {}

    def _get_cached_config(self, source_id: str):
        entry = self._config_cache.get(source_id)
        if entry is not None:
            config, expires_at = entry
            if time.monotonic() < expires_at:
                return config
        config = self.core.get_camera_config(source_id)
        if config:
            self._config_cache[source_id] = (config, time.monotonic() + _CONFIG_CACHE_TTL)
        return config

    def process(self, data: dict):
        source_id = data.get("source_id")
        image_key = data.get("image_key")

        config = self._get_cached_config(source_id)
        if not config:
            raise ValueError(f"Config not found for source {source_id}")

        mapping = config.get("field_mapping", {})
        if isinstance(mapping, str):
            mapping = json.loads(mapping)

        plate = self.parser.extract_plate(
            data.get("payload"), config.get("format"), mapping
        )

        suggestions = []
        if not plate or config.get("run_anpr"):
            try:
                img_bytes = self.minio.get_image(image_key)
                suggestions = self.anpr.recognize(img_bytes)
                if suggestions and not plate:
                    plate = suggestions[0]["plate"]
            except Exception as e:
                logger.error("AI Recognition failed", extra={"error": str(e), "image_key": image_key})

        if plate:
            final_event = {
                "camera_source_id": source_id,
                "camera_source_name": config.get("name", source_id),
                "camera_id": source_id,
                "plate": plate.upper().replace(" ", ""),
                "suggestions": json.dumps(suggestions), 
                "image_key": image_key,
                "timestamp": data.get("at"),
                "raw_payload": data.get("payload"),
            }
            self.core.send_event(final_event)
            logger.info("Successfully processed plate", extra={
                "plate": final_event["plate"],
                "source": final_event["camera_source_name"],
                "camera_id": final_event["camera_id"]
            })
