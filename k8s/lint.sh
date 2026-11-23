#!/usr/bin/env nix-shell
#!nix-shell -i bash -p kustomize kubeconform pluto conftest jq yamllint -I nixpkgs=channel:nixpkgs-unstable
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

TMP_DIR=""
cleanup() {
  if [ -n "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT

LOCAL_RUNNER="${HOME}/ghq/github.com/k-wa-wa/nuage-workspace/.github/workflows/validate/k8s/run.sh"
if [ -x "$LOCAL_RUNNER" ]; then
  RUNNER="$LOCAL_RUNNER"
else
  TMP_DIR="$(mktemp -d)"
  curl -sSfL "https://github.com/k-wa-wa/nuage-workspace/archive/refs/heads/master.tar.gz" \
    | tar -xz -C "$TMP_DIR" "nuage-workspace-master/.github/workflows/validate/k8s"
  RUNNER="${TMP_DIR}/nuage-workspace-master/.github/workflows/validate/k8s/run.sh"
fi

FAILED=0

if ! "$RUNNER" --yamllint-path k8s --prod k8s/overlays/prod; then
  FAILED=1
fi

if ! "$RUNNER" --preview k8s/overlays/preview; then
  FAILED=1
fi

exit "$FAILED"
