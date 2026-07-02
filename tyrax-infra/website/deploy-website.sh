#!/usr/bin/env bash
#
# Deploy tyrax-website static files to the tyrax.tech VPS.
#
#   WEBSITE_HOST=147.45.108.102 WEBSITE_USER=root bash deploy-website.sh
#
set -euo pipefail

WEBSITE_HOST="${WEBSITE_HOST:-147.45.108.102}"
WEBSITE_USER="${WEBSITE_USER:-root}"
REMOTE_PATH="${REMOTE_PATH:-/var/www/tyrax.tech}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WEBSITE_DIR="$REPO_ROOT/tyrax-website"

if [[ ! -f "$WEBSITE_DIR/index.html" ]]; then
  echo "tyrax-website not found at $WEBSITE_DIR" >&2
  exit 1
fi
if [[ ! -f "$WEBSITE_DIR/download/windows/TYRAX-Setup.exe" ]]; then
  echo "Missing TYRAX-Setup.exe — run tyrax-windows/build/stage-website-download.ps1 first." >&2
  exit 1
fi

echo "==> rsync $WEBSITE_DIR -> ${WEBSITE_USER}@${WEBSITE_HOST}:${REMOTE_PATH}/"
rsync -avz --delete \
  -e ssh \
  "$WEBSITE_DIR/" \
  "${WEBSITE_USER}@${WEBSITE_HOST}:${REMOTE_PATH}/"

echo "DONE. https://tyrax.tech/download/windows/TYRAX-Setup.exe"
