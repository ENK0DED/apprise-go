import json
import os
import sys
from pathlib import Path

script_dir = Path(__file__).resolve().parent
repo_root = script_dir.parents[2]
providers_root = repo_root / "internal" / "parity" / "providers"

sys.path.insert(0, str(script_dir))

from apprise.common import NotifyType  # noqa: E402
from capture_request import capture_request  # noqa: E402

DEFAULT_ENV = {
    "APPRISE_FIXED_TIME": "2024-01-01T00:00:00Z",
    "APPRISE_OAUTH_NONCE": "parity-nonce",
    "APPRISE_OAUTH_TIMESTAMP": "1704067200",
    "APPRISE_VAPID_TEST_JWT": "parity.jwt.token",
    "APPRISE_VAPID_TEST_PUBLIC_KEY": "parity-public-key",
    "APPRISE_VAPID_TEST_ENCRYPTED": "cGFyaXR5LXZhcGlk",
}


def apply_default_env():
    for key, value in DEFAULT_ENV.items():
        os.environ.setdefault(key, value)


def parse_notify_type(raw):
    value = (raw or "").strip().lower()
    if value == "success":
        return NotifyType.SUCCESS
    if value == "warning":
        return NotifyType.WARNING
    if value == "failure":
        return NotifyType.FAILURE
    return NotifyType.INFO


def main():
    apply_default_env()

    parity_root = repo_root / "internal" / "parity"
    os.chdir(parity_root)

    provider_dirs = [p for p in providers_root.iterdir() if p.is_dir()]
    if not provider_dirs:
        raise SystemExit(f"No provider dirs found under {providers_root}")

    for provider_dir in sorted(provider_dirs):
        cases_path = provider_dir / "cases.json"
        if not cases_path.exists():
            continue
        cases = json.loads(cases_path.read_text())
        if not cases:
            raise SystemExit(f"No cases in {cases_path}")

        golden_cases = []
        for case in cases:
            specs = capture_request(
                case["url"],
                case.get("body", ""),
                case.get("title", ""),
                parse_notify_type(case.get("type")),
            )
            golden_cases.append({"name": case["name"], "requests": specs})

        golden_path = provider_dir / "golden.json"
        golden_path.write_text(json.dumps(golden_cases, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
