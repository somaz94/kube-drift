# CLAUDE.md — kube-drift

Kubernetes operator that detects configuration drift between desired-state manifests and the live cluster on a schedule. The in-cluster counterpart to the `kube-diff` CLI. Kubebuilder v4 project (controller-runtime).

> **Maturity**: early scaffold (v0.1.0). The `DriftCheck` CRD and operator skeleton install and run, but the reconcile logic is a **stub** — no drift detection yet. Do not assume working comparison behavior when editing.

<br/>

## Build & Test

```bash
make manifests generate   # Regenerate CRD YAML, RBAC, DeepCopy after editing api/v1alpha1/types.go
make build                # Build manager binary → ./bin/manager
make test                 # Run unit tests with envtest
make lint                 # Run golangci-lint
make docker-build         # Build Docker image
make docker-buildx        # Build and push multi-arch image
make install / uninstall  # Install / remove CRD in the cluster
make deploy / undeploy    # Deploy / remove controller
make version              # Show current version
make bump-version VERSION_BUMP=vX.Y.Z   # Bump version across all files
```

<br/>

## Project Structure

```
cmd/main.go                        # Controller-runtime manager entry point
api/v1alpha1/
  types.go                         # DriftCheck spec/status (apiGroup drift.somaz.io, v1alpha1)
  groupversion_info.go             # GroupVersion registration
  zz_generated.deepcopy.go         # Generated DeepCopy methods
internal/controller/
  driftcheck_controller.go         # Reconciler — STUB (Phase 2 wires it to the kube-diff engine)
  driftcheck_controller_test.go
config/
  crd/bases/                       # Generated CRD YAML
  default/                         # Main Kustomize overlay
  manager/                         # Deployment manifest
  rbac/                            # ClusterRole, bindings, service account
  samples/driftcheck_v1alpha1_sample.yaml
helm/kube-drift/                   # Helm chart (crds/ must match config/crd/bases/)
hack/                              # boilerplate.go.txt, bump-version.sh
```

<br/>

## Key Concepts

- **CRD**: `DriftCheck` (apiGroup `drift.somaz.io`, version `v1alpha1`).
  - `spec.source` — `{ type: Git|ConfigMap, git: {url,ref,path}, configMap: {name,namespace,key} }` — desired plain-YAML manifests.
  - `spec.target` — `{ namespaces: [], labelSelector: {} }` — narrows which live resources are compared.
  - `spec.interval` — re-evaluation cadence (default `5m`).
  - `status` — `lastCheckedAt`, `driftedResources[]` ({apiVersion,kind,name,namespace,status}), `summary` {changed,new,deleted,unchanged}, `observedGeneration`, `conditions`.
  - Per-resource drift status enum: `unchanged | changed | new | deleted`.
- **kube-diff dependency (Phase 2)**: the reconcile logic will import `github.com/somaz94/kube-diff/pkg/{engine,diff,source,cluster}` and call `engine.Compare(...)` → `[]*diff.Result`, then map results into `DriftCheck` status. This import is **not wired yet** — the controller is a stub. Do not reimplement comparison here; reuse the kube-diff engine.
- **v0.1 scope**: ConfigMap sources first; Git sources come later.
- **Envtest**: unit tests use controller-runtime envtest (fake API server).
- **Distroless**: image uses `gcr.io/distroless/static:nonroot`.
- **git-cliff**: release notes generated from conventional commits.

<br/>

## Phase 2 Plan (reconcile logic)

The current `Reconcile` only fetches the `DriftCheck` and logs. To implement drift detection:

1. Load desired manifests from `spec.source` (start with `ConfigMap`).
2. Read live objects scoped by `spec.target`.
3. Call `engine.Compare(...)` from kube-diff → `[]*diff.Result`.
4. Classify each result (changed/new/deleted/unchanged), populate `status.driftedResources` + `status.summary`, stamp `lastCheckedAt` and `observedGeneration`.
5. Requeue after `spec.interval`.

<br/>

## Common Pitfalls

- **Helm CRD sync**: CRDs in `helm/kube-drift/crds/` must match `config/crd/bases/` after any CRD change.
- **Codegen**: always run `make manifests generate` after editing `api/v1alpha1/types.go`.
- **e2e is gated**: `test-e2e.yml` triggers on `workflow_dispatch` only — the stub controller has no behavior to exercise end-to-end. Restore push/PR triggers when Phase 2 (test/e2e + test/utils + metrics RBAC) lands.
- **ARG re-declaration**: build args must be re-declared after each `FROM` in the multi-stage Dockerfile.

<br/>

## Conventions

- **Commits**: single-line English [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`). No `Co-Authored-By`.
- **Documentation & code comments**: English only.
- **Branches**: `feat/name`, `fix/name`.
- Do not push to remote unless explicitly requested.
