#!/usr/bin/env bash
# Run a command through the local SOCKS proxy (default: 127.0.0.1:1080).
set -euo pipefail

export ALL_PROXY="${ALL_PROXY:-socks5://127.0.0.1:1080}"
export HTTPS_PROXY="${HTTPS_PROXY:-socks5://127.0.0.1:1080}"
export HTTP_PROXY="${HTTP_PROXY:-socks5://127.0.0.1:1080}"

exec "$@"
