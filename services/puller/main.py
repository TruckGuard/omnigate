import os
import time
import logging
from redis import Redis
from src.config import cfg
from src.worker.puller_worker import PullWorker

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.redis import RedisInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.instrumentation.logging import LoggingInstrumentor


def init_otel(service_name: str) -> None:
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")
    resource = Resource(attributes={"service.name": service_name})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    RedisInstrumentor().instrument()
    RequestsInstrumentor().instrument()
    LoggingInstrumentor().instrument(set_logging_format=False)


def main():
    logging.basicConfig(level=logging.INFO)
    logger = logging.getLogger("omnigate-puller")

    try:
        init_otel("omnigate-puller")
        logger.info("OpenTelemetry initialised")
    except Exception as exc:
        logger.warning(f"OpenTelemetry init failed (tracing disabled): {exc}")

    redis = Redis.from_url(f"redis://{cfg.REDIS_ADDR}", decode_responses=True)
    worker = PullWorker()

    GROUP = "puller-workers"

    while True:
        try:
            redis.xgroup_create(cfg.STREAM_PULLER, GROUP, id="0", mkstream=True)
            logger.info(f"Consumer group '{GROUP}' ready on {cfg.STREAM_PULLER}")
            break
        except Exception as e:
            if "BUSYGROUP" in str(e):
                logger.info(f"Consumer group '{GROUP}' already exists")
                break
            logger.error(f"Failed to create consumer group: {e}. Retrying in 5s...")
            time.sleep(5)

    consumer_name = f"puller-{os.getpid()}"
    retry_counts: dict = {}

    while True:
        try:
            streams = redis.xreadgroup(
                GROUP,
                consumer_name,
                {cfg.STREAM_PULLER: ">"},
                count=1,
                block=5000,
            )

            if not streams:
                continue

            for _, messages in streams:
                for msg_id, data in messages:
                    raw = data.get("data", "")

                    try:
                        worker.process(raw)
                        redis.xack(cfg.STREAM_PULLER, GROUP, msg_id)
                        retry_counts.pop(msg_id, None)
                    except Exception as e:
                        logger.error(f"Failed to process {msg_id}: {e}")

                        retries = retry_counts.get(msg_id, 0) + 1
                        retry_counts[msg_id] = retries

                        if retries >= 3:
                            logger.error(f"Message {msg_id} failed 3 times, moving to DLQ")
                            redis.xadd(cfg.STREAM_DLQ, {
                                "msg_id": msg_id,
                                "stream": cfg.STREAM_PULLER,
                                "data": raw,
                                "error": str(e),
                            })
                            redis.xack(cfg.STREAM_PULLER, GROUP, msg_id)
                            retry_counts.pop(msg_id, None)

        except Exception as e:
            err_msg = str(e)
            if "NOGROUP" in err_msg:
                logger.warning("Consumer group missing, attempting to recreate...")
                try:
                    redis.xgroup_create(cfg.STREAM_PULLER, GROUP, id="0", mkstream=True)
                except Exception:
                    pass
            else:
                logger.critical(f"Redis connection error: {e}")
            time.sleep(5)


if __name__ == "__main__":
    main()
