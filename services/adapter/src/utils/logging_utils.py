import logging
import json
import sys
from datetime import datetime

class JsonFormatter(logging.Formatter):
    def __init__(self, service_name):
        super().__init__()
        self.service_name = service_name

    def format(self, record):
        log_record = {
            "time": self.formatTime(record, self.datefmt),
            "level": record.levelname,
            "msg": record.getMessage(),
            "service": self.service_name,
            "logger": record.name,
        }


        # Add extra fields but filter out standard ones
        standard_attrs = {
            "name", "msg", "args", "levelname", "levelno", "pathname", "filename",
            "module", "exc_info", "exc_text", "stack_info", "lineno", "funcName",
            "created", "msecs", "relativeCreated", "thread", "threadName",
            "processName", "process", "message"
        }
        
        for key, value in record.__dict__.items():
            if key not in standard_attrs and not key.startswith("_"):
                log_record[key] = value

        if record.exc_info:
            log_record["exception"] = self.formatException(record.exc_info)

        return json.dumps(log_record)

    def formatTime(self, record, datefmt=None):
        dt = datetime.fromtimestamp(record.created)
        return dt.isoformat() + "Z"

def setup_logging(service_name="truckguard-worker"):
    # Clear existing handlers from root logger to avoid duplicates
    root = logging.getLogger()
    if root.hasHandlers():
        for handler in root.handlers[:]:
            root.removeHandler(handler)
            
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JsonFormatter(service_name))
    root.addHandler(handler)
    root.setLevel(logging.INFO)
    
    return logging.getLogger(service_name)

# logger = setup_logging("truckguard-camera-adapter")
logger = logging.getLogger()