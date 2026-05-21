# 📡 TruckGuard Puller Worker

### 1. What is it?

The **Puller Worker** is an asynchronous background worker service written in **Python**. It executes triggered data-pull operations (polling external APIs or capturing camera frames) when requested by downstream actions.

### 2. Purpose & How it Works

- **Stream Consumer**:
  - Listens to the Valkey stream `events:puller` (under the `puller-workers` consumer group).
- **External Capturing & Polling**:
  - Supports fetching data from HTTP URLs (JSON/XML/Text API payloads).
  - Supports capturing snapshots from live camera streams (RTSP/MJPEG stream frame grab).
- **Data Re-injection**:
  - Forwards the captured data and binary image files back to the **Ingestor Service** via POST `/ingest/event`.
  - Attaches the matching `transaction_id` and target `source_id` (using the `ingest:assume-source` permission granted by its `PULLER_API_KEY`) to seamlessly continue active transactions.
- **Robustness**:
  - Implements automatic retries with dead-letter queueing (`events:dlq`) after 3 consecutive failures.

### 3. Tech Stack

- **Language**: Python 3.11+
- **Stream Processing**: [Valkey Streams](https://valkey.io/)
- **Libraries**: `requests` (HTTP client), `opencv-python` / `pillow` (for frame extraction)
- **Observability**: [OpenTelemetry](https://opentelemetry.io/)

### 4. Getting Started

#### **Prerequisites**

- Python 3.11+
- Valkey
- Access to Ingestor API

#### **Run Commands**

1.  **Install dependencies:**
    ```bash
    pip install -r requirements.txt
    ```
2.  **Start the worker:**
    ```bash
    python main.py
    ```

### 5. Configuration (Environment Variables)

```env
VALKEY_ADDR=localhost:6380
INGESTOR_URL=http://localhost:8090/ingest
PULLER_API_KEY=your_puller_api_key
STORAGE_ENDPOINT=localhost:3900
STORAGE_ACCESS_KEY=your_access_key
STORAGE_SECRET_KEY=your_secret_key
STORAGE_BUCKET=truckguard-images
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```
