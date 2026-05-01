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
            region="garage"
        )
        self.bucket = cfg.STORAGE_BUCKET

    def get_image(self, image_key: str) -> bytes:
        """Завантажує зображення з MinIO та повертає байти"""
        try:
            response = self.client.get_object(self.bucket, image_key)
            data = response.read()
            response.close()
            response.release_conn()
            return data
        except Exception as e:
            logger.error("MinIO error", extra={"image_key": image_key, "error": str(e)})
            raise