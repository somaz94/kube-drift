# CLAUDE.md — kube-drift

A Kubernetes controller built with Kubebuilder (controller-runtime).

## Build & Test

```bash
make build           # Build manager binary
make test            # Run unit tests with envtest
make test-e2e        # Run e2e tests (requires Kind cluster)
make test-helm       # Run Helm chart tests
make lint            # Run golangci-lint
make manifests       # Generate CRD, RBAC manifests
make generate        # Generate DeepCopy methods
make docker-build    # Build Docker image
make docker-buildx   # Build and push multi-arch image
make deploy          # Deploy to cluster
make undeploy        # Remove from cluster
make version         # Show current version
make bump-version VERSION_BUMP=vX.Y.Z  # Bump version across all files
```

## Project Structure

```
cmd/main.go                    # Entry point (controller-runtime manager)
api/v1/
  types.go                     # CRD spec/status definitions
  groupversion_info.go         # GroupVersion registration
internal/controller/
  driftcheck_controller.go     # Reconciler logic
  driftcheck_controller_test.go
config/
  crd/bases/                   # Generated CRD YAML
  default/                     # Kustomize overlay (namespace, patches)
  manager/                     # Deployment manifest
  rbac/                        # RBAC roles and bindings
  samples/                     # Example CR YAML
helm/kube-drift/             # Helm chart
hack/
  boilerplate.go.txt           # License header for generated code
  bump-version.sh              # Version bump across all files
```

## Key Concepts

- **CRD**: DriftCheck (apiGroup: drift.somaz.io/v1)
- **Reconciler**: Watches DriftCheck, reconciles desired state
- **Kustomize**: config/default builds full deployment manifests
- **Envtest**: Unit tests use controller-runtime envtest (fake API server)
- **Distroless**: Docker image uses gcr.io/distroless/static:nonroot
- **Helm**: Chart in helm/kube-drift/ with CRD sync from config/crd/bases/
- **git-cliff**: Release notes generated from conventional commits

## Common Pitfalls

- Helm CRD sync: CRDs in `helm/kube-drift/crds/` must match `config/crd/bases/`
- ARG re-declaration: Build args must be re-declared after each FROM in multi-stage Dockerfile
- Version bump: `make bump-version` auto-updates Makefile, Chart.yaml, values.yaml, kustomization.yaml

## Release Workflow

```
make bump-version VERSION_BUMP=vX.Y.Z  # Update versions
git commit -am "chore: bump version to vX.Y.Z"
git push origin main
make docker-buildx               # Build and push multi-arch image
git tag vX.Y.Z && git push origin vX.Y.Z  # Triggers release + helm-release workflows
```
