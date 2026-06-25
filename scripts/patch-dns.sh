#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="bare-web-proxy-cluster"

echo "=== Ensuring correct kubectl context ==="
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "=== Patching CoreDNS ConfigMap to forward DNS to host gateway (192.168.5.1) ==="
kubectl get configmap coredns -n kube-system -o yaml | sed 's/forward \. \/etc\/resolv\.conf/forward . 192.168.5.1/' | kubectl apply -f -

echo "=== Restarting CoreDNS deployment ==="
kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

echo "=== DNS Patch Applied Successfully ==="
