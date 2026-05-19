import cv2
import logging

logger = logging.getLogger(__name__)

_OPEN_TIMEOUT_MS = 10_000
_READ_TIMEOUT_MS = 10_000


def grab_frame_jpeg(rtsp_url: str) -> bytes:
    """Open an RTSP stream, grab one frame, return JPEG bytes.

    Uses the bundled FFmpeg backend inside opencv-python-headless so no
    system FFmpeg installation is required.
    """
    cap = cv2.VideoCapture(rtsp_url, cv2.CAP_FFMPEG)
    cap.set(cv2.CAP_PROP_OPEN_TIMEOUT_MSEC, _OPEN_TIMEOUT_MS)
    cap.set(cv2.CAP_PROP_READ_TIMEOUT_MSEC, _READ_TIMEOUT_MS)

    try:
        if not cap.isOpened():
            raise RuntimeError(f"Could not open RTSP stream: {rtsp_url}")

        ret, frame = cap.read()
        if not ret or frame is None:
            raise RuntimeError(f"No frame received from RTSP stream: {rtsp_url}")

        ok, buf = cv2.imencode(".jpg", frame)
        if not ok:
            raise RuntimeError("Failed to JPEG-encode RTSP frame")

        logger.info("Captured RTSP frame", extra={"url": rtsp_url, "shape": frame.shape})
        return buf.tobytes()
    finally:
        cap.release()
