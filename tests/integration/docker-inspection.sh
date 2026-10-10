#!/bin/sh
# Fixtures live in a unique Compose project; cleanup never touches other stacks.
set -eu
project="faultharbor-test-$(date +%s)-$$"
result=$(mktemp)
compose() { docker compose -p "$project" -f tests/integration/compose.yaml "$@"; }
cleanup() { compose down --volumes --remove-orphans >/dev/null; rm -f "$result"; }
trap cleanup EXIT INT TERM
compose up -d
# Health checks and restart backoff need time to produce observations. Retry
# assertions within a bounded window instead of assuming a fixed sleep is enough.
attempt=0
while [ "$attempt" -lt 30 ]; do
  attempt=$((attempt + 1))
  if compose exec -T consumer faultharbor inspect docker >"$result"; then
    if python3 tests/integration/verify.py "$result" "$project"; then
      # The consumer must have no access to the Docker socket.
      compose exec -T consumer test ! -S /var/run/docker.sock
      echo "Docker inspection integration passed."
      exit 0
    fi
  fi
  sleep 1
done
echo "Docker inspection fixtures did not reach the expected state." >&2
exit 1
