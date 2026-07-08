# template-go-k8s-tool

A GitHub template repository for building Kubernetes controllers with [Kubebuilder](https://kubebuilder.io/) (controller-runtime), Helm charts, Docker, and automated CI/CD workflows.

<br/>

## What's Included

| Category | Files | Description |
|----------|-------|-------------|
| **Controller** | `cmd/`, `api/v1/`, `internal/controller/` | Controller-runtime manager with example CRD and reconciler |
| **CRD** | `config/crd/` | Example CustomResourceDefinition with spec/status |
| **K8s Config** | `config/` | Kustomize overlays (default, manager, rbac, samples) |
| **Helm** | `helm/` | Helm chart with values, templates, CRDs |
| **Docker** | `Dockerfile`, `.dockerignore` | Multi-stage build (golang → distroless:nonroot) |
| **Build** | `Makefile` | build, test, lint, manifests, generate, deploy, docker, pr |
| **CI/CD** | `.github/workflows/` | Test, e2e, lint, release, helm-release, changelog, contributors |
| **Scripts** | `scripts/`, `hack/` | PR auto-generator, version bump, helm tests |
| **Docs** | `CLAUDE.md`, `docs/` | Project guidelines and development guide |

<br/>

## Quick Start

<br/>

### 1. Create from Template

Click **"Use this template"** on GitHub, or:

```bash
gh repo create my-controller --template somaz94/template-go-k8s-tool --public --clone
cd my-controller
```

<br/>

### 2. Replace Placeholders

| Placeholder | Replace With | Example |
|-------------|-------------|---------|
| `somaz94` | Your GitHub username | `somaz94` |
| `kube-drift` | Your repository name | `my-controller` |
| `somaz.io` | Your CRD domain | `example.dev` |
| `drift` | Your CRD API group | `apps` |
| `somaz940` | Your Docker registry | `somaz940` |
| `backup6695808` | Your GitLab group (for mirror) | `backup6695808` |
| `DriftCheck` | Your CRD kind name | `AppConfig` |
| `driftcheck` | Your CRD kind (lowercase) | `appconfig` |

Quick replace:

```bash
# macOS
find . -type f -not -path './.git/*' -exec sed -i '' \
  -e 's/somaz94/somaz94/g' \
  -e 's/kube-drift/my-controller/g' \
  -e 's/somaz.io/example.dev/g' \
  -e 's/drift/apps/g' \
  -e 's/somaz940/somaz940/g' \
  -e 's/backup6695808/backup6695808/g' \
  -e 's/DriftCheck/AppConfig/g' \
  -e 's/driftcheck/appconfig/g' {} +

# Rename CRD file
mv config/crd/bases/drift.somaz.io_driftchecks.yaml \
   config/crd/bases/apps.example.dev_appconfigs.yaml

# Rename helm chart directory
mv helm/kube-drift helm/my-controller
```

<br/>

### 3. Initialize Module

```bash
go mod init github.com/somaz94/kube-drift
go mod tidy
```

<br/>

### 4. Generate & Build

```bash
make manifests generate   # Generate CRD YAML + DeepCopy
make build                # Build binary → ./bin/manager
make test                 # Run unit tests
```

<br/>

## Project Structure

```
.
├── cmd/
│   └── main.go                          # Controller-runtime manager entry point
├── api/
│   └── v1/
│       ├── types.go                     # CRD Spec/Status definitions
│       └── groupversion_info.go         # GroupVersion registration
├── internal/
│   └── controller/
│       ├── driftcheck_controller.go     # Reconciler logic
│       └── driftcheck_controller_test.go
├── config/
│   ├── crd/
│   │   └── bases/                       # Generated CRD YAML
│   ├── default/
│   │   ├── kustomization.yaml           # Main kustomize overlay
│   │   └── manager_metrics_patch.yaml
│   ├── manager/
│   │   ├── kustomization.yaml
│   │   └── manager.yaml                 # Deployment manifest
│   ├── rbac/
│   │   ├── kustomization.yaml
│   │   ├── role.yaml                    # ClusterRole
│   │   ├── role_binding.yaml
│   │   └── service_account.yaml
│   └── samples/
│       └── driftcheck_v1_sample.yaml    # Example CR
├── helm/
│   └── kube-drift/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── .helmignore
│       ├── crds/
│       └── templates/
├── hack/
│   ├── boilerplate.go.txt               # License header for generated code
│   └── bump-version.sh                  # Version bump across all files
├── scripts/
│   └── create-pr.sh                     # Auto-generate PR body
├── .github/
│   ├── workflows/
│   │   ├── test.yml                     # CI: test + manifests verify
│   │   ├── test-e2e.yml                 # E2E tests with Kind cluster
│   │   ├── lint.yml                     # golangci-lint
│   │   ├── release.yml                  # GitHub release (git-cliff) + major tag
│   │   ├── helm-release.yml             # Helm chart release to gh-pages
│   │   ├── changelog-generator.yml
│   │   ├── contributors.yml
│   │   ├── dependabot-auto-merge.yml
│   │   ├── stale-issues.yml
│   │   ├── issue-greeting.yml
│   │   └── gitlab-mirror.yml
│   ├── dependabot.yml
│   └── release.yml
├── .dockerignore
├── .gitattributes
├── .gitignore
├── .golangci.yml
├── cliff.toml
├── Dockerfile                           # Multi-stage (golang → distroless)
├── Makefile
├── CLAUDE.md
├── LICENSE
├── PROJECT                              # Kubebuilder project metadata
├── docs/
│   └── DEVELOPMENT.md
├── go.mod
└── README.md
```

<br/>

## Key Differences from CLI Template

| | `template-go-cli` | `template-go-k8s-tool` |
|---|---|---|
| Framework | Cobra CLI | controller-runtime (Kubebuilder) |
| Entry point | CLI commands | Controller manager + reconciler |
| Distribution | GoReleaser + Homebrew | Docker image + Kustomize + Helm |
| Config | CLI flags + YAML file | CRD + Kustomize overlays |
| Docker base | None | `distroless:nonroot` |
| Testing | `go test` | envtest + e2e (Kind) |
| Linting | None | golangci-lint |
| Makefile | build, test, pr | + manifests, generate, deploy, lint, version |
| Code gen | None | controller-gen (CRD, RBAC, DeepCopy) |
| Release notes | GoReleaser | git-cliff |

<br/>

## Makefile Targets

```bash
make help              # Show all targets
make build             # Build binary → ./bin/manager
make test              # Run unit tests with envtest
make test-e2e          # Run e2e tests (requires Kind)
make test-helm         # Run Helm chart tests
make lint              # Run golangci-lint
make manifests         # Generate CRD YAML, RBAC roles
make generate          # Generate DeepCopy methods
make fmt               # Format code
make vet               # Run go vet
make docker-build      # Build Docker image
make docker-push       # Push Docker image
make docker-buildx     # Build and push multi-arch image
make install           # Install CRDs into cluster
make uninstall         # Remove CRDs from cluster
make deploy            # Deploy controller to cluster
make undeploy          # Remove controller from cluster
make version           # Show current version
make bump-version VERSION_BUMP=vX.Y.Z  # Bump version
make branch name=x     # Create feature branch feat/x
make pr title="..."    # Test → push → create PR
make clean             # Remove build artifacts
make install-tools     # Install all required tools
```

<br/>

## CI/CD Workflows

| Workflow | Trigger | Description |
|----------|---------|-------------|
| `test.yml` | push, PR, dispatch | Unit tests → Manifests verify |
| `test-e2e.yml` | push, PR, dispatch | E2E tests with Kind cluster |
| `lint.yml` | dispatch | golangci-lint |
| `release.yml` | tag push `v*` | GitHub release (git-cliff) + major tag update |
| `helm-release.yml` | tag push `v*` | Helm chart release to gh-pages |
| `changelog-generator.yml` | after release, PR merge | Auto-generate CHANGELOG.md |
| `contributors.yml` | after changelog | Auto-generate CONTRIBUTORS.md |
| `dependabot-auto-merge.yml` | dependabot PR | Auto-merge minor/patch updates |
| `stale-issues.yml` | daily cron | Auto-close stale issues (30d + 7d) |
| `issue-greeting.yml` | issue opened | Welcome message |
| `gitlab-mirror.yml` | push to main | Mirror to GitLab |

<br/>

### Workflow Chain

```
tag push v* → Create release (git-cliff) + update major tag (v1)
            → Helm chart release (gh-pages)
                └→ Generate changelog
                      └→ Generate Contributors
```

<br/>

## GitHub Secrets Required

| Secret | Usage |
|--------|-------|
| `PAT_TOKEN` | Release, helm release, contributors (cross-repo access) |
| `GITLAB_TOKEN` | GitLab mirror (optional) |

<br/>

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`)
- **CRD changes**: Always run `make manifests generate` after modifying `api/v1/types.go`
- **Branches**: `feat/name`, `fix/name`
- **paths-ignore**: CI skips `.github/workflows/**` and `**/*.md` changes

<br/>

## License

See [LICENSE](LICENSE) — replace with your chosen license.
