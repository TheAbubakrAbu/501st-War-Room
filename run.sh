#!/usr/bin/env bash
# run.sh
#
# Runs the 501st War Room locally with the token from .env. Settings go to
# data.nosync, which iCloud skips, and go run builds outside the project.
# One-time setup: chmod +x run.sh

set -euo pipefail

cd "$(dirname "$0")"
set -a
source .env
set +a
exec go run ./cmd/warroom --data data.nosync
