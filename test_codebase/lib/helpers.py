import json
from typing import Dict, Any


def load_config(path: str) -> Dict[str, Any]:
    with open(path, "r") as f:
        return json.load(f)


def validate_email(email: str) -> bool:
    return "@" in email and "." in email.split("@")[1]
