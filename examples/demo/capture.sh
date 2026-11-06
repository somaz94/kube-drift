#!/usr/bin/env bash
# Runs the README demo for real on a throwaway kind cluster, with kube-drift
# installed from helm/kube-drift and its published image, and saves each command's
# output under examples/demo/captured/, which replay.sh prints back.
# Usage: examples/demo/capture.sh   (needs kind, kubectl and helm; pulls the image)
set -euo pipefail

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
ROOT_DIR="$(cd "${DEMO_DIR}/../.." && pwd)"
OUT="${DEMO_DIR}/captured"
CLUSTER="kube-drift-demo"

for tool in kind kubectl helm; do
    command -v "${tool}" >/dev/null 2>&1 || { echo "${tool} not found" >&2; exit 1; }
done
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"; then
    echo "kind cluster ${CLUSTER} already exists; delete it first" >&2
    exit 1
fi

WORK="$(mktemp -d)"
# kind would otherwise switch the current context of the user's own kubeconfig.
export KUBECONFIG="${WORK}/kubeconfig"
cleanup() {
    kind delete cluster --name "${CLUSTER}" --kubeconfig "${KUBECONFIG}" >/dev/null 2>&1 || true
    rm -rf "${WORK:?}"
}
trap cleanup EXIT

# Wait until the DriftCheck reports a summary reaches the expected value.
wait_for() {
    for _ in $(seq 1 90); do
        [ "$(kubectl get driftcheck web -o jsonpath="$1")" = "$2" ] && return 0
        sleep 1
    done
    echo "timed out waiting for $1 = $2" >&2
    kubectl get driftcheck web -o yaml >&2
    exit 1
}

kind create cluster --name "${CLUSTER}" --kubeconfig "${KUBECONFIG}" --wait 60s
# The view role lets the controller read Deployments, not only ConfigMaps and Secrets.
helm install kube-drift "${ROOT_DIR}/helm/kube-drift" --namespace kube-drift-system --create-namespace \
    --set rbac.viewRole.enabled=true --wait --timeout 3m

cd "${DEMO_DIR}"
kubectl apply -f desired.yaml
kubectl create configmap desired-manifests --from-file=manifests.yaml=desired.yaml
kubectl apply -f driftcheck.yaml
wait_for '{.status.summary.unchanged}' 2

rm -rf "${OUT}"
mkdir -p "${OUT}"
kubectl get driftcheck > "${OUT}/01-kubectl" 2>&1
kubectl scale deployment web --replicas=5 > "${OUT}/02-kubectl" 2>&1
kubectl delete configmap web-config > "${OUT}/03-kubectl" 2>&1
# Off screen: the next scheduled check, at most one interval later.
wait_for '{.status.summary.changed}/{.status.summary.new}' 1/1
kubectl get driftcheck > "${OUT}/04-kubectl" 2>&1

echo "captured into ${OUT}"
