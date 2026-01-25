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
