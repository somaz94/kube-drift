#!/bin/bash
# Validate the kube-drift Helm chart: lint, template render, package, and CRD
# presence. The chart currently ships the DriftCheck CRD only — controller
# deployment/RBAC templates are not authored yet — so this checks chart
# packaging rather than a live install. Run `make test-e2e` for a live,
# kustomize-based deployment test.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0
CHART_DIR="helm/kube-drift"
CRD_NAME="driftchecks.drift.somaz.io"

log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS + 1)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL + 1)); }
log_skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; SKIP=$((SKIP + 1)); }

echo ""
log_info "========================================="
log_info "kube-drift Helm Chart Test"
log_info "========================================="
log_info "NOTE: the chart ships the DriftCheck CRD only; controller deployment"
log_info "templates are not authored yet, so this validates chart packaging"
log_info "(lint/template/package) rather than a live install."
echo ""

if ! command -v helm >/dev/null 2>&1; then
  log_skip "helm not installed — skipping chart tests"
  echo -e "  ${YELLOW}SKIP: 1${NC}"
  exit 0
fi

log_info "Linting Helm chart..."
if helm lint "${CHART_DIR}" >/dev/null 2>&1; then
  log_pass "helm lint"
else
  log_fail "helm lint"
  helm lint "${CHART_DIR}" || true
fi

log_info "Testing Helm template rendering..."
if helm template test "${CHART_DIR}" >/dev/null 2>&1; then
  log_pass "helm template"
else
  log_fail "helm template"
fi

log_info "Testing Helm package..."
PACKAGE_DIR=$(mktemp -d)
if helm package "${CHART_DIR}" -d "${PACKAGE_DIR}" >/dev/null 2>&1; then
  log_pass "helm package"
else
  log_fail "helm package"
fi
rm -rf "${PACKAGE_DIR}"

log_info "Checking the DriftCheck CRD ships with the chart..."
if grep -q "name: ${CRD_NAME}" "${CHART_DIR}"/crds/*.yaml 2>/dev/null; then
  log_pass "DriftCheck CRD present"
else
  log_fail "DriftCheck CRD present"
fi

echo ""
log_info "--- Summary ---"
echo -e "  ${GREEN}PASS: ${PASS}${NC}"
echo -e "  ${RED}FAIL: ${FAIL}${NC}"
echo -e "  ${YELLOW}SKIP: ${SKIP}${NC}"

[ "${FAIL}" -eq 0 ]
