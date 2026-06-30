from __future__ import annotations

from fnmatch import fnmatch


def domain_allowed(hostname: str, patterns: list[str]) -> bool:
    normalized = hostname.lower().strip(".")
    for pattern in patterns:
        candidate = pattern.lower().strip(".")
        if candidate.startswith("*."):
            suffix = candidate[2:]
            if normalized == suffix or normalized.endswith(f".{suffix}"):
                return True
        if fnmatch(normalized, candidate):
            return True
    return False
