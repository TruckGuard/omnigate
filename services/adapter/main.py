import os
import time
from redis import Redis
from src.config import cfg
from src.utils.logging_utils import setup_logging
from src.clients.core_client import CoreClient
from src.clients.puller_client import PullerClient
from src.clients.anpr_client import ANPRClient
from src.clients.minio_client import MinioStorage
from src.logic.processor import EventProcessor

# ── OpenTelemetry setup ────────────────────────────────────────────────────────
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.redis import RedisInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.instrumentation.logging import LoggingInstrumentor
from opentelemetry._logs import set_logger_provider
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import OTLPLogExporter
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
import logging
import json


def init_otel(service_name: str) -> None:
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
    resource = Resource(attributes={"service.name": service_name})
    
    # Tracing
    trace_provider = TracerProvider(resource=resource)
    trace_exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    trace_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
    trace.set_tracer_provider(trace_provider)
    
    # Logging
    log_provider = LoggerProvider(resource=resource)
    set_logger_provider(log_provider)
    log_exporter = OTLPLogExporter(endpoint=endpoint, insecure=True)
    log_provider.add_log_record_processor(BatchLogRecordProcessor(log_exporter))
    
    # Add handler to root logger
    handler = LoggingHandler(level=logging.NOTSET, logger_provider=log_provider)
    logging.getLogger().addHandler(handler)

    RedisInstrumentor().instrument()
    RequestsInstrumentor().instrument()
    LoggingInstrumentor().instrument(set_logging_format=False)


def main():
    logger = setup_logging("omnigate-adapter")

    try:
        init_otel("omnigate-adapter")
        logger.info("OpenTelemetry initialised")
    except Exception as exc:
        logger.warning(f"OpenTelemetry init failed (tracing disabled): {exc}")

    redis = Redis.from_url(f"redis://{cfg.REDIS_ADDR}", decode_responses=True)
    core = CoreClient()
    puller = PullerClient(redis)
    storage = MinioStorage()
    anpr = ANPRClient()

    processor = EventProcessor(core, puller, storage, anpr, redis)

    # Create consumer group with retries
    while True:
        try:
            redis.xgroup_create(cfg.STREAM_RAW, "adapter-workers", id="0", mkstream=True)
            logger.info(f"Consumer group 'adapter-workers' created for stream {cfg.STREAM_RAW}")
            break
        except Exception as e:
            if "BUSYGROUP" in str(e):
                logger.info("Consumer group 'adapter-workers' already exists")
                break
            logger.error(f"Failed to create consumer group: {e}. Retrying in 5s...")
            time.sleep(5)

    consumer_name = f"adapter-{os.getpid()}"
    last_id = ">"
    retry_counts = {}

    while True:
        try:
            streams = redis.xreadgroup(
                "adapter-workers",
                consumer_name,
                {cfg.STREAM_RAW: last_id},
                count=1,
                block=5000,
            )

            if not streams:
                continue

            for _, messages in streams:
                for msg_id, data in messages:
                    raw = data.get("data", "")

                    try:
                        # Extract Trace Context from stream message
                        carrier = {}
                        try:
                            msg_json = json.loads(raw)
                            if "trace_context" in msg_json:
                                carrier = {"traceparent": msg_json["trace_context"]}
                        except Exception:
                            pass
                        
                        extracted_context = TraceContextTextMapPropagator().extract(carrier=carrier)
                        
                        tracer = trace.get_tracer(__name__)
                        with tracer.start_as_current_span("adapter-process", context=extracted_context):
                            processor.process(raw)
                        
                        redis.xack(cfg.STREAM_RAW, "adapter-workers", msg_id)
                        retry_counts.pop(msg_id, None)
                    except Exception as e:
                        logger.error(f"Failed to process {msg_id}: {e}")

                        retries = retry_counts.get(msg_id, 0) + 1
                        retry_counts[msg_id] = retries

                        if retries >= 3:
                            logger.error(f"Message {msg_id} failed 3 times, moving to DLQ")
                            redis.xadd(cfg.STREAM_DLQ, {
                                "msg_id": msg_id,
                                "stream": cfg.STREAM_RAW,
                                "data": raw,
                                "error": str(e),
                            })
                            redis.xack(cfg.STREAM_RAW, "adapter-workers", msg_id)
                            retry_counts.pop(msg_id, None)

        except Exception as e:
            err_msg = str(e)
            if "NOGROUP" in err_msg:
                logger.warning("Consumer group missing, attempting to recreate...")
                try:
                    redis.xgroup_create(cfg.STREAM_RAW, "adapter-workers", id="0", mkstream=True)
                except Exception:
                    pass
            else:
                logger.critical(f"Redis connection error: {e}")
            time.sleep(5)


if __name__ == "__main__":
    main()