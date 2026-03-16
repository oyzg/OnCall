import logging


def setup_logging(level: str) -> logging.Logger:
    normalized = getattr(logging, level.upper(), logging.INFO)
    logging.basicConfig(
        level=normalized,
        format="%(asctime)s [%(levelname)s] %(name)s %(message)s",
    )
    return logging.getLogger("python-ai")
