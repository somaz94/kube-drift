# CLAUDE.md — kube-drift

Kubernetes operator that detects configuration drift between desired-state manifests and the live cluster on a schedule. The in-cluster counterpart to the `kube-diff` CLI. Kubebuilder v4 project (controller-runtime).

> **Maturity**: working operator (v0.1.x released; v0.3 features on `main`). Drift detection runs against `ConfigMap`, `Git`, `Helm`, and `Kustomize` sources, records results into `status`, exposes Prometheus metrics, and sends webhook notifications. Git clones are anonymous by default; private repos are supported via `source.git.auth` (Basic/Bearer/SSH, go-git, credentials from a Secret in the DriftCheck's namespace).

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
  driftcheck_controller.go         # Reconciler — source → kube-diff engine → status + metrics
  notify.go                        # Webhook notification dispatch (dedup, resolved transition)
  *_test.go
internal/source/                   # Git clone + in-process Helm/Kustomize rendering
internal/metrics/                  # kube_drift_resources gauge
internal/notify/                   # Slack/Generic webhook sender
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
  - `spec.source` — `{ type: Git|ConfigMap|Helm|Kustomize, git, configMap, helm, kustomize }`. `Helm`/`Kustomize` source their files from a nested `git` block and render in-process. `spec.notify.webhooks[]` (Slack/Generic, url or urlSecretRef) sends a message when the drift state changes (deduped via `status.lastNotifiedHash`).
  - `spec.target` — `{ namespaces: [], labelSelector: {} }` — narrows which live resources are compared.
  - `spec.interval` — re-evaluation cadence (default `5m`).
  - `status` — `lastCheckedAt`, `driftedResources[]` ({apiVersion,kind,name,namespace,status}), `summary` {changed,new,deleted,unchanged}, `observedGeneration`, `lastNotifiedHash`, `conditions`.
  - Per-resource drift status enum: `unchanged | changed | new | deleted`.
- **kube-diff dependency**: the reconcile logic imports `github.com/somaz94/kube-diff/pkg/{engine,diff,source,cluster}` and calls `engine.Run(...)` → `[]*diff.Result`, then maps results into `DriftCheck` status. Do not reimplement comparison here; reuse the kube-diff engine.
- **In-process rendering**: `internal/source` renders Helm charts (Helm SDK: `loader`/`chartutil`/`engine`) and Kustomize overlays (`krusty`/`filesys`) from a Git checkout — **no shell-out** (kube-diff's own `helm`/`kustomize` sources shell out and are unused here). `withCheckout` in `git.go` is the shared clone+cleanup harness.
- **Envtest**: unit tests use controller-runtime envtest (fake API server).
- **Distroless**: image uses `gcr.io/distroless/static:nonroot`.
- **git-cliff**: release notes generated from conventional commits.

<br/>

## Reconcile flow

`Reconcile` (in `driftcheck_controller.go`):

1. Load desired manifests from `spec.source` via `buildSource` (ConfigMap / Git / Helm / Kustomize).
2. Read live objects via the kube-diff cluster fetcher (built from the manager's rest.Config).
3. Call `engine.Run(...)` from kube-diff → `[]*diff.Result`.
4. Classify each result (changed/new/deleted/unchanged), populate `status.driftedResources` + `status.summary`, stamp `lastCheckedAt` and `observedGeneration`, set the `Ready` condition.
5. Record metrics, dispatch webhook notifications (`notify.go`, deduped via `status.lastNotifiedHash`), requeue after `spec.interval`.

<br/>

## Common Pitfalls

- **Helm CRD sync**: CRDs in `helm/kube-drift/crds/` must match `config/crd/bases/` after any CRD change.
- **Codegen**: always run `make manifests generate` after editing `api/v1alpha1/types.go`.
- **Helm chart RBAC sync**: `helm/kube-drift/templates/rbac.yaml` (hand-written) must match the generated `config/rbac/role.yaml` after any `+kubebuilder:rbac` marker change.
- **In-process rendering, not shell-out**: Helm/Kustomize sources render with the Helm SDK / Kustomize API in-process. Never shell out to `helm`/`kustomize` — they are absent from the distroless image. (kube-diff's own `pkg/source/{helm,kustomize}.go` DO shell out; those are unused here.)
- **ARG re-declaration**: build args must be re-declared after each `FROM` in the multi-stage Dockerfile.

<br/>

## Conventions

- **Commits**: single-line English [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`). No `Co-Authored-By`.
- **Documentation & code comments**: English only.
- **Branches**: `feat/name`, `fix/name`.
- Do not push to remote unless explicitly requested.
