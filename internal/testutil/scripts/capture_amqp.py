"""Send through the actual Python AMQP adapter to a local test broker."""

import argparse
import datetime
import json
import os

import apprise
from apprise import AppriseAsset
from apprise.common import NotifyFormat
import apprise.plugins.amqp.base as amqp_base


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--body", default="")
    parser.add_argument("--title", default="")
    parser.add_argument("--body-format", default="")
    args = parser.parse_args()
    fixed = os.environ.get("APPRISE_FIXED_TIME")
    if fixed:
        timestamp = datetime.datetime.fromisoformat(fixed.replace("Z", "+00:00")).timestamp()
        amqp_base.time = lambda: timestamp
    asset_kwargs = {}
    if args.body_format:
        asset_kwargs["body_format"] = NotifyFormat(args.body_format)
    app = apprise.Apprise(asset=AppriseAsset(**asset_kwargs))
    if not app.add(args.url):
        raise ValueError("Python AMQP target did not initialize")
    print(json.dumps({"success": bool(app.notify(body=args.body, title=args.title))}))


if __name__ == "__main__":
    main()
