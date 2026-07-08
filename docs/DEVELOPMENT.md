# Development

Guide for building, testing, and contributing to this Kubernetes controller.

<br/>

## Prerequisites

- Go 1.26+
- Docker
- kubectl
- Make
- Kind (for e2e testing)
- Helm (for chart testing)

<br/>

## Build

```bash
make build           # Build binary → ./bin/manager
make docker-build    # Build Docker image
make docker-buildx   # Build and push multi-arch image
make clean           # Remove build artifacts
```

<br/>

## Code Generation

```bash
make manifests       # Generate CRD YAML, RBAC roles (controller-gen)
make generate        # Generate DeepCopy methods (controller-gen)
```

Always run these after modifying `api/v1/types.go` or RBAC markers.

<br/>

## Testing

```bash
make test            # Run unit tests with envtest
make test-e2e        # Run e2e tests (requires Kind cluster)
make test-helm       # Run Helm chart tests
make lint            # Run golangci-lint
```

<br/>

## Deployment

```bash
make install         # Install CRDs into cluster
make deploy          # Deploy controller to cluster
make undeploy        # Remove controller from cluster
make uninstall       # Remove CRDs from cluster
```

<br/>

## Version Management

```bash
make version                           # Show current version
make bump-version VERSION_BUMP=v0.2.0  # Bump across all files
```

<br/>

## Workflow

```bash
make check-gh        # Verify gh CLI is installed and authenticated
make branch name=feature-name   # Create feature branch from main
make pr title="feat: add feature"   # Test → push → create PR
```

<br/>

## CI/CD Workflows

| Workflow | Trigger | Description |
|----------|---------|-------------|
| `test.yml` | push, PR, dispatch | Unit tests → Manifests verify |
| `test-e2e.yml` | push, PR, dispatch | E2E tests with Kind cluster |
| `lint.yml` | dispatch | golangci-lint |
| `release.yml` | tag push `v*` | GitHub release (git-cliff) + major tag |
| `helm-release.yml` | tag push `v*` | Helm chart release to gh-pages |
| `changelog-generator.yml` | after release, PR merge | Auto-generate CHANGELOG.md |
| `contributors.yml` | after changelog | Auto-generate CONTRIBUTORS.md |
| `stale-issues.yml` | daily cron | Auto-close stale issues |
| `dependabot-auto-merge.yml` | PR (dependabot) | Auto-merge minor/patch updates |
| `issue-greeting.yml` | issue opened | Welcome message |
| `gitlab-mirror.yml` | push to main | Mirror to GitLab |

### Workflow Chain

```
tag push v* → Create release (git-cliff) + update major tag (v1)
            → Helm chart release (gh-pages)
                └→ Generate changelog
                      └→ Generate Contributors
```

<br/>

## Release

```bash
# 1. Bump version across all files
make bump-version VERSION_BUMP=v0.2.0

# 2. Commit and push
git commit -am "chore: bump version to v0.2.0"
git push origin main

# 3. Build and push Docker image
make docker-buildx

# 4. Tag and push (triggers release + helm-release workflows)
git tag v0.2.0
git push origin v0.2.0
```

<br/>

## Conventions

- **Commits**: Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`)
- **CRD changes**: Always run `make manifests generate` after modifying types.go
- **Helm CRDs**: Keep `helm/kube-drift/crds/` in sync with `config/crd/bases/`
- **paths-ignore**: CI skips `.github/workflows/**` and `**/*.md` changes
- **Docker**: Multi-arch builds with distroless nonroot base
- **Linting**: golangci-lint with project `.golangci.yml` config
