#!/bin/sh
set -eu

python3 /app/formula-worker/worker.py &
FORMULA_WORKER_PID=$!
/app/stock-web &
APP_PID=$!

cleanup() {
  kill "$APP_PID" 2>/dev/null || true
  kill "$FORMULA_WORKER_PID" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

wait "$APP_PID"
