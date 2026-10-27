#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."

python3 -m venv .venv
. .venv/bin/activate

python -m pip install --upgrade pip
if [[ -f ../apprise/pyproject.toml ]]; then
  python -m pip install -e "../apprise[all-plugins]"
else
  python -m pip install "apprise[all-plugins]"
fi

# AMQP 1.0 is intentionally excluded from upstream's all-plugins extra.
# Its real broker parity cases require the source-built Proton adapter.
python -m pip install "python-qpid-proton==0.40.0"
python -c 'import pika, proton'
