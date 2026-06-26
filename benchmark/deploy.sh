#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="bare-web-proxy-cluster"
IMAGE_NAME="bare-web-proxy:local"
MOCK_IMAGE_NAME="bare-web-proxy-mock:local"

echo "=== Go modules tidy & vendor ==="
go mod tidy
go mod vendor

echo "=== Docker build (Proxy & Mock Server) ==="
docker build -t "$IMAGE_NAME" .
docker build -t "$MOCK_IMAGE_NAME" -f benchmark/mockserver/Dockerfile .

# Ensure kubectl is using the correct context (assuming cluster exists)
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "=== Load images to kind ==="
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"
kind load docker-image "$MOCK_IMAGE_NAME" --name "$CLUSTER_NAME"

echo "=== Deploy to Kubernetes (Local Overlay with Mock) ==="
kubectl apply -k k8s/overlays/local
kubectl rollout restart deployment/bare-web-proxy

echo "=== Wait for deployment to be ready ==="
kubectl rollout status deployment/bare-web-proxy --timeout=120s

echo "=== Setup Port Forwarding ==="
# Kill any existing port-forwarding for these services
pgrep -f "port-forward svc/bare-web-proxy-service" | xargs -r kill || true
pgrep -f "port-forward svc/bare-web-proxy-mock-service" | xargs -r kill || true

echo "Starting port-forward in background (port 3000 -> 80, 3003 -> 3003)..."
# Write logs to workspace to avoid temp directories outside the workspace
kubectl port-forward svc/bare-web-proxy-service 3000:80 --address 0.0.0.0 > "$(pwd)/port-forward.log" 2>&1 &
PID_PROXY=$!
kubectl port-forward svc/bare-web-proxy-mock-service 3003:3003 --address 0.0.0.0 > "$(pwd)/port-forward-mock.log" 2>&1 &
PID_MOCK=$!

# Wait briefly and verify it is running
sleep 2
if ps -p $PID_PROXY > /dev/null && ps -p $PID_MOCK > /dev/null; then
  echo "Port forward started successfully."
  echo "Proxy: http://localhost:3000"
  echo "Mock Server: http://localhost:3003"
else
  echo "Failed to start port-forward. Check logs:"
  echo "Proxy Log:"
  cat "$(pwd)/port-forward.log" || true
  echo "Mock Log:"
  cat "$(pwd)/port-forward-mock.log" || true
  exit 1
fi
