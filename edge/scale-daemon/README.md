Edge-демон: читає TCP-вагу, стабілізує сигнал, відправляє `{"weight_kg": N}` в OmniGate Ingestor.

## Конфігурація

Всі параметри — змінні середовища. CLI-прапорці мають вищий пріоритет над env.

| Змінна | Прапорець | За замовчуванням | Опис |
|---|---|---|---|
| `SCALE_HOST` | `--scale-host` | `127.0.0.1` | TCP-хост ваги |
| `SCALE_PORT` | `--scale-port` | `5001` | TCP-порт ваги |
| `INGESTOR_URL` | `--ingestor-url` | `http://localhost:8090/ingest/event` | URL Ingestor (через NGINX) |
| `API_KEY` | `--api-key` | — | API-токен пристрою |
| `DEVICE_ID` | `--device-id` | — | Назва пристрою (тільки для логів) |
| `OTEL_ENDPOINT` | `--otel-endpoint` | `localhost:4318` | OTLP HTTP колектор |
| `DEBOUNCE_MS` | `--debounce-ms` | `2000` | Мс стабільності перед відправкою |
| `MIN_WEIGHT_KG` | `--min-weight-kg` | `0` | Ігнорувати показання нижче порогу |
| `RECONNECT_SEC` | `--reconnect-sec` | `5` | Затримка перепідключення до ваги |

## Запуск

**Вручну (для тестування):**
```bash
SCALE_HOST=192.168.1.100 \
SCALE_PORT=5001 \
INGESTOR_URL=http://omnigate.example.com:8090/ingest/event \
API_KEY=your-api-key \
./scale-daemon
```

**Через systemd (продакшн):**
```bash
# 1. Встановити
sudo ./install.sh

# 2. Виставити конфіг
sudo nano /etc/omnigate/scale.env

# 3. Перезапустити
sudo systemctl restart scale-daemon
```

`/etc/omnigate/scale.env`:
```env
SCALE_HOST=192.168.1.100
SCALE_PORT=5001
INGESTOR_URL=http://omnigate.example.com:8090/ingest/event
API_KEY=your-api-key
DEVICE_ID=scale-gate-01
MIN_WEIGHT_KG=200
DEBOUNCE_MS=3000
```

**Логи:**
```bash
sudo journalctl -u scale-daemon -f
```

## Збірка

```bash
go build -o scale-daemon .

# Raspberry Pi
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o scale-daemon-linux-arm64 .
```
