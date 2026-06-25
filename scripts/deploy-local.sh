#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="bare-web-proxy-cluster"
IMAGE_NAME="bare-web-proxy:local"

echo "=== Go modules tidy & vendor ==="
go mod tidy
go mod vendor

echo "=== Docker build ==="
docker build -t "$IMAGE_NAME" .

echo "=== Check kind cluster ==="
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  echo "Creating kind cluster ${CLUSTER_NAME}..."
  kind create cluster --name "$CLUSTER_NAME"
else
  echo "kind cluster ${CLUSTER_NAME} already exists."
fi

# Ensure kubectl is using the correct context
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "=== Pull chromedp/headless-shell:latest ==="
docker pull chromedp/headless-shell:latest

echo "=== Load images to kind ==="
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"
kind load docker-image chromedp/headless-shell:latest --name "$CLUSTER_NAME"

echo "=== Deploy to Kubernetes ==="
kubectl apply -k k8s/overlays/local
kubectl rollout restart deployment/bare-web-proxy

echo "=== Wait for deployment to be ready ==="
kubectl rollout status deployment/bare-web-proxy --timeout=120s

echo "=== Setup Port Forwarding ==="
# Kill any existing port-forwarding for this service
pgrep -f "port-forward svc/bare-web-proxy-service" | xargs -r kill || true

echo "Starting port-forward in background (port 3000 -> 80)..."
# Write logs to workspace to avoid temp directories outside the workspace
kubectl port-forward svc/bare-web-proxy-service 3000:80 > "$(pwd)/port-forward.log" 2>&1 &
PF_PID=$!

# Wait briefly and verify it is running
sleep 2
if ps -p $PF_PID > /dev/null; then
  echo "Port forward started successfully (PID: $PF_PID)."
  echo "You can access the proxy at http://localhost:3000/proxy?url=https://example.com"
else
  echo "Failed to start port-forward. Check log at $(pwd)/port-forward.log"
  cat "$(pwd)/port-forward.log"
  exit 1
fi
