class WorkerError(Exception):
    """Base error for local worker failures."""


class ConfigError(WorkerError):
    """Raised when local worker configuration is missing or invalid."""


class APIError(WorkerError):
    def __init__(self, code: str, message: str, retryable: bool = False, status: int | None = None):
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message
        self.retryable = retryable
        self.status = status
