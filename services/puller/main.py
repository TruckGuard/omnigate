import os
import uvicorn
from fastapi import FastAPI
from src.api.handlers import router
from src.config import cfg

# ── OpenTelemetry setup ────────────────────────────────────────────────────────
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
import logging

logger = logging.getLogger("omnigate-puller")

def init_otel(app: FastAPI, service_name: str) -> None:
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")
    resource = Resource(attributes={"service.name": service_name})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app)
    RequestsInstrumentor().instrument()


app = FastAPI(title="TruckGuard Puller Service")
app.include_router(router)

try:
    init_otel(app, "omnigate-puller")
    logger.info("OpenTelemetry initialised")
except Exception as exc:
    logger.warning(f"OpenTelemetry init failed (tracing disabled): {exc}")


@app.get("/health")
async def health():
    return {"status": "ok"}


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=cfg.PORT,
        reload=True,
    )
