#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "=== Go modules tidy & vendor ==="
go mod tidy
go mod vendor

echo "=== Build & start proxy / mock server via docker compose ==="
docker compose -f benchmark/docker-compose.yml up -d --build

echo "=== Wait for services to be ready ==="
for name_path in "proxy:http://localhost:3000/" "mock server:http://localhost:3003/simple"; do
  name="${name_path%%:*}"
  url="${name_path#*:}"
  ready=0
  for _ in $(seq 1 30); do
    if curl -sf "${url}" -o /dev/null; then
      ready=1
      break
    fi
    sleep 1
  done
  if [ "${ready}" -ne 1 ]; then
    echo "Timed out waiting for ${name} (${url})."
    docker compose -f benchmark/docker-compose.yml logs
    exit 1
  fi
done

echo "=== Ready ==="
echo "Proxy: http://localhost:3000"
echo "Mock Server: http://localhost:3003"
