# ⚙️ TruckGuard Adapter Worker

### 1. What is it?

The **Adapter Worker** is a high-performance asynchronous processing engine written in **Python**. It acts as a unified "bridge" between raw hardware data (cameras, scales) and the backend, orchestrating recognition, parsing, and data transformation.

### 2. Purpose & How it Works

It operates as a background consumer for all hardware events:

1.  **Consume**: Listens to the `events:adapter` Redis Stream for all incoming ingestion events.
2.  **Route**: Dispatches processing based on the event `type` (`camera` or `weight`).
3.  **Process (Camera)**:
    - Forwards images to the **ANPR Service** for hardware-agnostic license plate recognition.
    - Decodes manufacturer-specific payloads.
4.  **Process (Weight)**:
    - Extracts weight values from sensor payloads based on configuration.
5.  **Finalize**: Enriches events with metadata and matches them with active **Sticky Transactions** in the **Core Service**.
6.  **Reliability**: Implements a Dead Letter Queue (`events:dlq`) for handling processing failures.


### 3. Tech Stack

- **Language**: [Python 3.12+](https://www.python.org/)
- **Stream Processing**: [Valkey Streams](https://valkey.io/)
- **Infrastructure**: [Valkey](https://valkey.io/), [Garage Storage](https://garagehq.deuxfleurs.fr/)
- **Observability**: [OpenTelemetry](https://opentelemetry.io/)

### 4. Getting Started

#### **Prerequisites**

- Python 3.12+
- Valkey
- Access to Core & ANPR APIs
- Access to Garage Storage

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
CORE_URL=http://localhost:8090/api/v1
ANPR_URL=http://localhost:8000
STORAGE_ENDPOINT=localhost:3900
STORAGE_ACCESS_KEY=your_access_key
STORAGE_SECRET_KEY=your_secret_key
STORAGE_BUCKET=truckguard-images
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```
