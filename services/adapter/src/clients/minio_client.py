import io

from minio import Minio
from src.config import cfg
from src.utils.logging_utils import logger


class MinioStorage:
    def __init__(self):
        self.client = Minio(
            cfg.STORAGE_ENDPOINT,
            access_key=cfg.STORAGE_ACCESS_KEY,
            secret_key=cfg.STORAGE_SECRET_KEY,
            secure=False,
            region="garage",
        )
        self.bucket = cfg.STORAGE_BUCKET

    def get_image(self, image_key: str) -> bytes:
        try:
            response = self.client.get_object(self.bucket, image_key)
            data = response.read()
            response.close()
            response.release_conn()
            return data
        except Exception as e:
            logger.error("MinIO get error", extra={"image_key": image_key, "error": str(e)})
            raise

    def upload_image(self, data: bytes, object_name: str, content_type: str = "image/jpeg") -> str:
        """Upload raw bytes to Garage/MinIO and return the object key."""
        try:
            self.client.put_object(
                self.bucket,
                object_name,
                io.BytesIO(data),
                length=len(data),
                content_type=content_type,
            )
            logger.debug("MinIO upload ok", extra={"object_name": object_name, "size": len(data)})
            return object_name
        except Exception as e:
            logger.error("MinIO upload error", extra={"object_name": object_name, "error": str(e)})
            raise