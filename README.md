# kube-drift

> Kubernetes operator that continuously detects configuration **drift** between your desired-state manifests and the live cluster, on a schedule. The in-cluster, GitOps-grade counterpart to the [`kube-diff`](https://github.com/somaz94/kube-diff) CLI.

![Top Language](https://img.shields.io/github/languages/top/somaz94/kube-drift?color=green&logo=go&logoColor=b)
![Version](https://img.shields.io/github/v/tag/somaz94/kube-drift?label=version&logo=kubernetes&logoColor=white)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/somaz94/kube-drift)](https://goreportcard.com/report/github.com/somaz94/kube-drift)
![GitHub Stars](https://img.shields.io/github/stars/somaz94/kube-drift?style=social)

> **Status: early development (v0.1.0 — WIP).** The `DriftCheck` CRD, the operator, and drift detection for both **`ConfigMap` and `Git` sources** are implemented: the controller loads the desired manifests, compares them against the live cluster via the `kube-diff` engine, and records the result into `status`. Still pending: Git credential support (clones are anonymous today) and a broader read-RBAC story for arbitrary compared kinds. See [Roadmap](#roadmap) before depending on this.

<br/>

## Overview

`kube-drift` runs the same manifest-vs-cluster comparison that the `kube-diff` CLI performs on demand, but does it **continuously and in-cluster** as a Kubernetes operator. You declare a `DriftCheck` resource that points at a set of desired-state manifests (from a ConfigMap or a Git repository) and, on a configurable interval, the controller compares them against the live objects in the cluster and reports what has drifted — resources that changed, that are missing, or that exist but were never declared.

Where `kube-diff` answers "does the cluster match this directory of YAML right now?" from a laptop or CI job, `kube-drift` answers "has the cluster drifted from its declared state since I last looked?" and keeps that answer fresh in the resource's `status`, making it a natural fit for GitOps and audit workflows.

<br/>

## Features

![DriftCheck CRD](https://img.shields.io/badge/DriftCheck_CRD-326CE5?logo=kubernetes&logoColor=white)
![ConfigMap Source](https://img.shields.io/badge/ConfigMap_Source-blue?logo=kubernetes&logoColor=white)
![Git Source](https://img.shields.io/badge/Git_Source-F05032?logo=git&logoColor=white)
![Helm Source](https://img.shields.io/badge/Helm_Source-0F1689?logo=helm&logoColor=white)
![Kustomize Source](https://img.shields.io/badge/Kustomize_Source-326CE5?logo=kubernetes&logoColor=white)
![Scheduled Reconcile](https://img.shields.io/badge/Scheduled_Reconcile-green?logo=kubernetes&logoColor=white)
![Status Reporting](https://img.shields.io/badge/Status_Reporting-green?logo=kubernetes&logoColor=white)
![Webhook Alerts](https://img.shields.io/badge/Webhook_Alerts-4A154B?logo=slack&logoColor=white)
![Kubebuilder](https://img.shields.io/badge/Kubebuilder_v4-teal?logo=kubernetes&logoColor=white)

- **`DriftCheck` CRD** (`drift.somaz.io/v1alpha1`) — declarative drift checks, one per desired-state source
- **Pluggable sources** — desired manifests come from a `ConfigMap`, a `Git` repository, or a `Helm` chart / `Kustomize` overlay rendered **in-process** (no `helm`/`kustomize` binary, no shell-out)
- **Scoped comparison** — narrow which live resources are compared via `target.namespaces` and `target.labelSelector`
- **Scheduled re-evaluation** — configurable `interval` (default `5m`) for continuous drift detection
- **Structured status** — per-resource drift entries plus a rolled-up summary (changed / new / deleted / unchanged), `lastCheckedAt`, `observedGeneration`, and standard conditions
- **Webhook notifications** — Slack or generic-JSON webhooks fire when the drift state changes (detected or resolved), deduplicated so they don't repeat on every re-check; the URL can come from a `Secret`
- **Shared engine** — reuses the comparison engine extracted from `kube-diff`, so CLI and operator produce consistent results

> Source-backend maturity: `ConfigMap`, `Git`, `Helm`, and `Kustomize` sources are implemented. `Helm`/`Kustomize` render from a Git checkout and are rendered in-process. Git clones **anonymously** for now (no credential support yet), so only publicly cloneable repositories work today — see [Roadmap](#roadmap).

<br/>

## How It Works

Each `DriftCheck` drives the following loop:

1. **Load desired state** — fetch plain-YAML manifests from the configured `source`: ConfigMap key(s), or a Git repository cloned at `ref` with manifests read from `path`.
2. **Read live state** — list the matching live objects, scoped by `target.namespaces` / `target.labelSelector`.
3. **Compare** — hand both sides to the `kube-diff` engine (`engine.Compare(...)`), producing a `[]*diff.Result`.
4. **Map to status** — classify each result as `changed`, `new`, `deleted`, or `unchanged`, write the drifted entries + summary into `.status`, and stamp `lastCheckedAt`.
5. **Requeue** — re-run after `spec.interval`.

<br/>

## Installation

<br/>

### Prerequisites

- Kubernetes v1.16+
- `kubectl` configured against the target cluster
- Helm 3 (for the chart install path)

<br/>

### Install with Kustomize

```bash
make install    # Install the DriftCheck CRD into the cluster
make deploy     # Deploy the controller (config/default overlay)
```

<br/>

### Install with Helm

The chart lives at [`helm/kube-drift/`](helm/kube-drift/):

```bash
helm install kube-drift ./helm/kube-drift \
  --namespace kube-drift-system --create-namespace
```

<br/>

## Usage

> Full walkthrough — install, both source types, reading results, metrics, RBAC, and troubleshooting — in the **[Usage Guide](docs/USAGE.md)**.

Create a `DriftCheck` that compares a set of desired manifests stored in a ConfigMap against the `default` namespace, re-checking every 5 minutes ([`config/samples/driftcheck_v1alpha1_sample.yaml`](config/samples/driftcheck_v1alpha1_sample.yaml)):

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-sample
  namespace: default
spec:
  # Desired-state manifests to compare against the live cluster.
  source:
    type: ConfigMap
    configMap:
      name: desired-manifests
      # namespace: default    # defaults to the DriftCheck's namespace
      # key: manifests.yaml    # omit to concatenate every key as a YAML stream
  # Optionally narrow which resources are compared.
  target:
    namespaces:
      - default
  # How often to re-evaluate drift.
  interval: 5m
```

```bash
kubectl apply -f config/samples/driftcheck_v1alpha1_sample.yaml
kubectl get driftchecks
```

Or point at a Git repository instead of a ConfigMap — cloned at `ref`, with plain YAML read from `path`:

```yaml
spec:
  source:
    type: Git
    git:
      url: https://github.com/somaz94/kube-drift.git   # anonymous clone (public repos only in v0.1)
      ref: main                                        # branch, tag, or commit; omit for the default branch
      path: config/samples                             # sub-directory holding the manifests; omit for the root
  interval: 10m
```

<br/>

### Spec Reference

| Field | Type | Description |
|---|---|---|
| `source.type` | enum | `Git` or `ConfigMap` — where desired manifests come from |
| `source.git.url` | string | Clone URL of the repository (required when `type: Git`) |
| `source.git.ref` | string | Branch, tag, or commit; defaults to the repo's default branch |
| `source.git.path` | string | Directory within the repo holding the manifests; defaults to root |
| `source.configMap.name` | string | ConfigMap name (required when `type: ConfigMap`) |
| `source.configMap.namespace` | string | ConfigMap namespace; defaults to the DriftCheck's namespace |
| `source.configMap.key` | string | Single data key; omit to concatenate every key as a YAML stream |
| `target.namespaces` | list | Restrict comparison to these namespaces |
| `target.labelSelector` | selector | Further restrict which desired manifests are compared |
| `interval` | duration | How often to re-evaluate drift (default `5m`) |

<br/>

### Status Reference

| Field | Type | Description |
|---|---|---|
| `lastCheckedAt` | time | When the drift check last completed |
| `driftedResources[]` | list | Resources whose live state differs from desired (`apiVersion`, `kind`, `name`, `namespace`, `status`) |
| `summary` | object | Tally across all compared resources: `changed`, `new`, `deleted`, `unchanged` |
| `observedGeneration` | int | `.metadata.generation` last reconciled |
| `conditions` | list | Standard condition array for the latest observations |

Per-resource `status` values: `unchanged`, `changed`, `new`, `deleted`.

<br/>

## Roadmap

- [x] `DriftCheck` CRD (`drift.somaz.io/v1alpha1`) — spec, status, printer columns
- [x] Operator skeleton — manager, RBAC, Kustomize overlays, Helm chart, CI/CD
- [x] Reconcile loop wired to the `kube-diff` engine (`engine.Run`) — drift recorded into `status`
- [x] `ConfigMap` source backend
- [x] `Git` source backend — clone a repo at `ref`, load plain YAML from `path` (anonymous clone; go-git, no shell-out)
- [x] `Helm` / `Kustomize` source backends — render a chart / build an overlay from a Git checkout **in-process** (Helm SDK + Kustomize API, no shell-out)
- [x] Webhook notifications — Slack / generic-JSON, deduplicated on drift-state change, URL sourced from a `Secret`
- [x] End-to-end test suite + `test-e2e.yml` on push/PR via [`kind-e2e-test-action`](https://github.com/somaz94/kind-e2e-test-action)
- [ ] Git credential support for private repositories
- [ ] Read-RBAC story for comparing arbitrary resource kinds (currently only `configmaps` is declared; broader read is granted at install time)

<br/>

## Architecture

`kube-drift` deliberately does not reimplement manifest comparison. The comparison engine was extracted into reusable packages in [`kube-diff`](https://github.com/somaz94/kube-diff), and this operator consumes them directly:

- `github.com/somaz94/kube-diff/pkg/engine` — the top-level `Compare(...)` entry point
- `github.com/somaz94/kube-diff/pkg/diff` — the `Result` type and diff classification
- `github.com/somaz94/kube-diff/pkg/source` — loading desired-state manifests
- `github.com/somaz94/kube-diff/pkg/cluster` — reading live cluster objects

This keeps the CLI and the operator behaviorally consistent: the same comparison that `kube-diff` prints to a terminal is the one `kube-drift` records into `DriftCheck` status.

<br/>

## Project Structure

```
.
├── cmd/
│   └── main.go                              # Controller-runtime manager entry point
├── api/
│   └── v1alpha1/
│       ├── types.go                         # DriftCheck spec/status definitions
│       ├── groupversion_info.go             # GroupVersion registration
│       └── zz_generated.deepcopy.go         # Generated DeepCopy methods
├── internal/
│   ├── controller/
│   │   ├── driftcheck_controller.go         # Reconciler — load source, run kube-diff engine, write status
│   │   └── driftcheck_controller_test.go
│   ├── source/                              # Git desired-state source (go-git clone → FileSource)
│   └── metrics/                             # kube_drift_resources drift gauge
├── config/
│   ├── crd/bases/                           # Generated CRD YAML
│   ├── default/                             # Main Kustomize overlay
│   ├── manager/                             # Deployment manifest
│   ├── rbac/                                # ClusterRole, bindings, service account
│   └── samples/
│       └── driftcheck_v1alpha1_sample.yaml  # Example DriftCheck
├── helm/
│   └── kube-drift/                          # Helm chart (CRDs synced from config/crd/bases/)
├── hack/                                    # boilerplate header, version bump
├── scripts/                                 # PR auto-generator
├── docs/
│   ├── USAGE.md                             # Install, source types, results, metrics, RBAC
│   └── DEVELOPMENT.md
├── Dockerfile                               # Multi-stage (golang → distroless:nonroot)
├── Makefile
├── PROJECT                                  # Kubebuilder v4 project metadata
├── go.mod
└── README.md
```

<br/>

## Development

Kubebuilder v4 project. Common targets:

```bash
make manifests generate   # Regenerate CRD YAML, RBAC, and DeepCopy after editing api/v1alpha1/types.go
make build                # Build the manager binary → ./bin/manager
make test                 # Run unit tests with envtest
make lint                 # Run golangci-lint
make docker-build         # Build the container image
make install / uninstall  # Install / remove the CRD in the cluster
make deploy / undeploy    # Deploy / remove the controller
make help                 # List all targets
```

> After changing `api/v1alpha1/types.go`, always run `make manifests generate` and keep `helm/kube-drift/crds/` in sync with `config/crd/bases/`.

The end-to-end workflow (`test-e2e.yml`) runs the kind-based e2e suite on push / PR / manual dispatch via [`kind-e2e-test-action`](https://github.com/somaz94/kind-e2e-test-action): it builds the image, loads it into a kind cluster, deploys the controller, and asserts a `DriftCheck` detects drift and exposes the `kube_drift_resources` metric.

<br/>

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`)
- **Documentation & code comments**: English only
- **CRD changes**: always run `make manifests generate` after editing `api/v1alpha1/types.go`
- **Branches**: `feat/name`, `fix/name`

<br/>

## License

See [LICENSE](LICENSE) — Apache 2.0.
