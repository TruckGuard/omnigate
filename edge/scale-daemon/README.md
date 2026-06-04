Edge-демон: читає TCP-вагу, стабілізує сигнал, відправляє `{"weight_kg": N}` в OmniGate Ingestor.

## Конфігурація

Пріоритет (зліва — вищий):

```
CLI flag  >  env var  >  config file  >  hardcoded default
```

### JSON config file (рекомендовано)

Скопіювати приклад і відредагувати:

```bash
cp config.example.json config.json
```

`config.json`:
```json
{
  "scale_host": "192.168.1.100",
  "scale_port": "5001",
  "ingestor_url": "http://omnigate.example.com:8090/ingest/event",
  "device_id": "scale-gate-01",
  "api_key": "your-api-key",
  "debounce_ms": 5000,
  "min_weight_kg": 500,
  "reconnect_sec": 5,
  "log_level": "info",
  "http_timeout_sec": 10
}
```

Запуск з файлом:
```bash
./scale-daemon --config config.json
# або через env:
CONFIG_FILE=config.json ./scale-daemon
```

### Всі параметри

| JSON-ключ | Env-змінна | CLI-прапорець | За замовчуванням | Опис |
|---|---|---|---|---|
| `scale_host` | `SCALE_HOST` | `--scale-host` | `127.0.0.1` | TCP-хост ваги |
| `scale_port` | `SCALE_PORT` | `--scale-port` | `5001` | TCP-порт ваги |
| `ingestor_url` | `INGESTOR_URL` | `--ingestor-url` | `http://localhost:8090/ingest/event` | URL Ingestor |
| `api_key` | `API_KEY` | `--api-key` | — | API-токен пристрою |
| `device_id` | `DEVICE_ID` | `--device-id` | — | Назва пристрою (для логів) |
| `otel_endpoint` | `OTEL_ENDPOINT` | `--otel-endpoint` | `localhost:4318` | OTLP HTTP колектор |
| `debounce_ms` | `DEBOUNCE_MS` | `--debounce-ms` | `2000` | Мс тиші перед відправкою піку |
| `min_weight_kg` | `MIN_WEIGHT_KG` | `--min-weight-kg` | `0` | Ігнорувати показання нижче порогу |
| `reconnect_sec` | `RECONNECT_SEC` | `--reconnect-sec` | `5` | Затримка перепідключення до ваги |
| `log_level` | `LOG_LEVEL` | `--log-level` | `info` | Рівень логів: debug, info, warn, error |
| `http_timeout_sec` | `HTTP_TIMEOUT_SEC` | `--http-timeout-sec` | `10` | Таймаут HTTP-запиту до ingestor |
| — | `CONFIG_FILE` | `--config` | `config.json` | Шлях до JSON config файлу |

## Запуск

**Вручну (для тестування):**
```bash
./scale-daemon --config config.json

# або без файлу — через прапорці:
./scale-daemon \
  --scale-host 192.168.1.100 \
  --ingestor-url http://omnigate.example.com:8090/ingest/event \
  --api-key your-api-key \
  --debounce-ms 5000
```

**Через systemd (продакшн):**
```bash
# 1. Встановити (копіює бінарник, config.json, реєструє сервіс)
sudo ./install.sh

# 2. Виставити конфіг (все в одному файлі)
sudo nano /etc/omnigate/scale-daemon.json

# 3. Перезапустити
sudo systemctl restart scale-daemon
```

**Логи:**
```bash
sudo journalctl -u scale-daemon -f
# debug-рівень (всі зміни ваги):
sudo journalctl -u scale-daemon -f | grep -v DEBUG
```

## Алгоритм відправки

Демон відправляє **максимальне** значення ваги після стабілізації:

1. Нове показання → оновлюється пік, перезапускається таймер `debounce_ms`
2. Якщо нових даних не було `debounce_ms` мс → відправляється пік
3. Якщо прийшло нове значення поки таймер тікає → таймер скидається
4. Якщо вага повернулась до нуля → сесія скидається (наступна фура — з нуля)

## Збірка

```bash
go build -o scale-daemon .

# Raspberry Pi / ARM
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o scale-daemon-linux-arm64 .

# x86_64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o scale-daemon-linux-amd64 .
```
