# kube-drift — Usage Guide

How to install `kube-drift`, declare a `DriftCheck`, and read the drift it detects. For an architectural overview see the [README](../README.md); for building and testing see [DEVELOPMENT.md](DEVELOPMENT.md).

<br/>

## Prerequisites

- A Kubernetes cluster (v1.16+) and `kubectl` pointed at it
- `make` and Go (only for the `make install` / `make deploy` build path)
- For a `Git` source: the cluster nodes must have outbound network access to the repository host

<br/>

## Install

Install the CRD and deploy the controller with the Kustomize path:

```bash
cd kube-drift
make install    # install the DriftCheck CRD
make deploy     # deploy the controller into the kube-drift-system namespace
```

Or install with Helm — the chart ships the CRD plus the controller Deployment, RBAC, ServiceAccount, and metrics Service:

```bash
helm install kube-drift ./helm/kube-drift \
  --namespace kube-drift-system --create-namespace
```

Verify the controller is running:

```bash
kubectl -n kube-drift-system get deploy,pod
```

<br/>

## Concepts

A `DriftCheck` (`drift.somaz.io/v1alpha1`) declares one drift comparison:

- **`spec.source`** — where the *desired* manifests come from:
  - `ConfigMap` — plain-YAML manifests stored in a ConfigMap's data key(s)
  - `Git` — plain-YAML manifests in a Git repository, cloned at a `ref` and read from a `path`
  - `Helm` — a Helm chart in a Git repository, rendered **in-process** with the given release name / namespace / values
  - `Kustomize` — a Kustomize overlay in a Git repository, built **in-process**
- **`spec.target`** — narrows which live resources are compared (`namespaces`, `labelSelector`). Empty means each manifest is matched by its own group/kind/namespace/name.
- **`spec.interval`** — how often the check re-runs (default `5m`).
- **`status`** — the result: a per-resource `driftedResources[]` list, a rolled-up `summary`, `lastCheckedAt`, and standard `conditions`.

On each interval the controller loads the desired manifests, compares them against the live cluster via the [`kube-diff`](https://github.com/somaz94/kube-diff) engine, and records what drifted.

<br/>

## Example 1 — ConfigMap source

A `ConfigMap` source works out of the box, because the controller ships with read access to `configmaps` (see [RBAC](#rbac-for-non-configmap-kinds) for other kinds).

**1. Store the desired manifests in a ConfigMap:**

```bash
kubectl create configmap desired-manifests -n default --from-file=manifests.yaml=/dev/stdin <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  key: desired-value
EOF
```

**2. Declare the DriftCheck:**

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-sample
  namespace: default
spec:
  source:
    type: ConfigMap
    configMap:
      name: desired-manifests
      # namespace: default    # defaults to the DriftCheck's namespace
      # key: manifests.yaml    # omit to concatenate every key as a YAML stream
  target:
    namespaces:
      - default
  interval: 5m
```

```bash
kubectl apply -f config/samples/driftcheck_v1alpha1_sample.yaml
```

Since `app-config` does not exist in the cluster yet, it surfaces as **new** (declared but absent). Create it with a different value and it becomes **changed**; create it identically and it becomes **unchanged**.

<br/>

## Example 2 — Git source

The desired manifests are cloned from a Git repository. Only **plain YAML** under `path` is loaded — for chart / overlay rendering use the `Helm` or `Kustomize` source instead (Examples 3 and 4).

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-git
  namespace: default
spec:
  source:
    type: Git
    git:
      url: https://github.com/somaz94/kube-drift.git   # anonymous by default; add auth for private repos (below)
      ref: main                                        # branch, tag, or commit SHA; omit for the default branch
      path: config/samples                             # sub-directory holding the manifests; omit for the repo root
  interval: 10m
```

- **`ref`** accepts a branch name, a tag, or a commit SHA. A non-default branch resolves via its `origin/<ref>` remote-tracking form.
- **`path`** is a sub-directory within the repository. Only `.yaml` / `.yml` files are parsed; other files are ignored.
- Clones are **anonymous by default**. To clone a private repository, add a `source.git.auth` block — see [Private repositories (Git auth)](#private-repositories-git-auth) below.

<br/>

## Private repositories (Git auth)

Set `source.git.auth` to clone a private repository. When `auth` is omitted the clone is anonymous (the default above). Authentication uses pure-Go [`go-git`](https://github.com/go-git/go-git) — there is no shell-out to a `git` binary. Credentials are read by the controller from a `Secret` **in the DriftCheck's namespace** (this is why the controller declares `secrets: get` RBAC).

`auth` has two fields:

- **`type`** — one of `Basic`, `Bearer`, or `SSH` (required).
- **`secretRef.name`** — the name of a `Secret` in the DriftCheck's namespace holding the credentials (required). The keys read from the Secret's `data` depend on `type` (below).

The same `auth` block is accepted on the nested Git blocks of the Helm and Kustomize sources — `source.helm.git.auth` and `source.kustomize.git.auth`.

If the referenced Secret is missing, or a required key is absent, the check fails with a `SourceError` condition and retries on the next `interval`. It does **not** silently fall back to an anonymous clone.

<br/>

### Basic (HTTPS username / password or PAT)

For HTTPS with a username and password, or a GitHub/GitLab personal access token (PAT). For a PAT, put the token in `password` and any non-empty value in `username` (e.g. your Git username).

```bash
kubectl -n default create secret generic git-basic-auth \
  --from-literal=username='example-user' \
  --from-literal=password='example-pat-value'
```

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-git-basic
  namespace: default
spec:
  source:
    type: Git
    git:
      url: https://github.com/example-org/private-manifests.git
      ref: main
      path: config/samples
      auth:
        type: Basic
        secretRef:
          name: git-basic-auth
  interval: 10m
```

Secret keys: `username`, `password`.

<br/>

### Bearer (HTTPS bearer token)

For an HTTPS bearer token — e.g. a GitHub App installation token or an OAuth token.

```bash
kubectl -n default create secret generic git-bearer-auth \
  --from-literal=bearerToken='example-bearer-token'
```

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-git-bearer
  namespace: default
spec:
  source:
    type: Git
    git:
      url: https://github.com/example-org/private-manifests.git
      ref: main
      path: config/samples
      auth:
        type: Bearer
        secretRef:
          name: git-bearer-auth
  interval: 10m
```

Secret key: `bearerToken`.

<br/>

### SSH (private key)

For SSH-based clone URLs (`git@github.com:example-org/private-manifests.git`). The login user defaults to `git`.

Host-key verification is **fail-closed**: the `known_hosts` key is **required**, and there is no insecure skip option. An SSH auth Secret without `known_hosts` is rejected.

```bash
kubectl -n default create secret generic git-ssh-auth \
  --from-file=identity=$HOME/.ssh/id_ed25519 \
  --from-file=known_hosts=$HOME/.ssh/known_hosts
  # optionally, if the private key is passphrase-protected:
  # --from-literal=password='example-passphrase'
```

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-git-ssh
  namespace: default
spec:
  source:
    type: Git
    git:
      url: git@github.com:example-org/private-manifests.git
      ref: main
      path: config/samples
      auth:
        type: SSH
        secretRef:
          name: git-ssh-auth
  interval: 10m
```

The Secret's `data` (base64-encoded by Kubernetes) holds:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-ssh-auth
  namespace: default
type: Opaque
stringData:
  identity: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...redacted...
    -----END OPENSSH PRIVATE KEY-----
  known_hosts: |
    github.com ssh-ed25519 AAAA...redacted...
  # password: example-passphrase    # optional — only if the private key is encrypted
```

Secret keys: `identity` (PEM private key, required), `known_hosts` (required), `password` (private-key passphrase, optional).

<br/>

## Example 3 — Helm source

Render a Helm chart from a Git repository **in-process** (no `helm` binary in the controller image) and compare the rendered manifests against the cluster.

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-helm
  namespace: default
spec:
  source:
    type: Helm
    helm:
      git:
        url: https://github.com/example/charts.git
        ref: main
        path: charts/myapp        # directory containing Chart.yaml
      releaseName: myapp           # .Release.Name; defaults to the DriftCheck name
      namespace: default           # .Release.Namespace; defaults to the DriftCheck namespace
      valuesFiles:                 # merged in order, relative to the chart directory
        - values-prod.yaml
      values:                      # inline overrides applied last (highest precedence)
        replicaCount: 3
        image:
          tag: v1.4.0
  interval: 10m
```

- Values precedence: chart defaults → `valuesFiles` (in order) → inline `values`.
- Rendering uses the Helm SDK render engine; `NOTES.txt` and template partials (`_*.tpl`) are excluded, matching `helm template`.
- By default the chart must be **self-contained** — chart dependencies are expected to be vendored under `charts/`. This is the reproducible, GitOps-idiomatic default.

For a chart whose dependencies are declared in `Chart.yaml` but not vendored, set `source.helm.dependencyBuild: true` to fetch them into `charts/` before rendering (using the Helm SDK in-process, no shell-out):

```yaml
spec:
  source:
    type: Helm
    helm:
      git:
        url: https://github.com/example/charts.git
        ref: main
        path: charts/myapp
      dependencyBuild: true        # fetch declared-but-unvendored dependencies before rendering
  interval: 10m
```

- Default is `false`. Enabling it makes the controller reach each dependency's repository **over the network on every render**, so the dependency repositories must be HTTP(S) URLs reachable from the pod.
- Named `@alias` repositories are **not** supported; `oci://` dependency URLs are resolved natively.

<br/>

## Example 4 — Kustomize source

Build a Kustomize overlay from a Git repository **in-process** (no `kustomize` / `kubectl` binary in the controller image).

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-kustomize
  namespace: default
spec:
  source:
    type: Kustomize
    kustomize:
      git:
        url: https://github.com/example/config.git
        ref: main
        path: overlays/prod        # directory containing kustomization.yaml
  interval: 10m
```

- Uses the default (root-only) load restrictions — a kustomization cannot read files outside its own directory tree.
- Remote bases referenced by URL are fetched anonymously (they do not use the `source.kustomize.git.auth` credentials, which apply only to the top-level overlay clone).

<br/>

## Reading drift results

```bash
kubectl get driftchecks
# NAME                 DRIFTED   NEW   LAST CHECKED   AGE
# driftcheck-sample    0         1     10s            30s
```

The printer columns are `Drifted` (`summary.changed`), `New` (`summary.new`), `Last Checked`, and `Age`. For the full picture:

```bash
kubectl get driftcheck driftcheck-sample -o jsonpath='{.status.summary}'
#   {"changed":0,"new":1,"deleted":0,"unchanged":0}

kubectl get driftcheck driftcheck-sample -o jsonpath='{.status.driftedResources}'
#   [{"apiVersion":"v1","kind":"ConfigMap","name":"app-config","namespace":"default","status":"new"}]

kubectl describe driftcheck driftcheck-sample   # includes conditions and events
```

- **`status.summary`** — tally across every compared resource: `changed` / `new` / `deleted` / `unchanged`.
- **`status.driftedResources[]`** — only the resources that drifted (unchanged ones are omitted). Each `status` is `changed`, `new`, or `deleted`.
- **`status.conditions`** — a `Ready` condition, `True` on a successful evaluation or `False` with a reason on failure.
- **`status.lastCheckedAt`** — timestamp of the last completed evaluation.

Per-resource `status` meanings: `changed` (exists on both sides, differs), `new` (declared, missing live), `deleted` (live, never declared), `unchanged` (in sync).

<br/>

## Metrics

The controller exposes a Prometheus gauge per DriftCheck:

```
kube_drift_resources{driftcheck="driftcheck-sample", namespace="default", status="new"} 1
```

The `status` label is one of `changed` / `new` / `deleted` / `unchanged`. The metrics server is **secure** (HTTPS on port `8443`, requiring authentication/authorization), so scraping needs an authenticated client — e.g. a Prometheus `ServiceMonitor` with a bearer token, or the built-in `kube-drift-metrics-reader` ClusterRole bound to the scraping identity.

<br/>

## RBAC for non-ConfigMap kinds

The controller only declares read access to `configmaps` by default. To compare other kinds (Deployments, Services, …), the operator's ServiceAccount needs read access to them.

**Helm install (recommended).** The chart exposes two opt-in RBAC knobs, both off by default:

- `rbac.viewRole.enabled=true` binds the controller ServiceAccount to the built-in Kubernetes `view` ClusterRole — broad read access for comparing arbitrary kinds. `view` is read-only and excludes Secrets and RBAC objects by design.
- `rbac.extraRules` takes a list of standard RBAC policy rules; when non-empty, a dedicated read-only ClusterRole (+ binding) is created. Use read verbs only (`get`/`list`/`watch`).

```bash
# Bind the built-in view ClusterRole:
helm install kube-drift ./helm/kube-drift \
  --namespace kube-drift-system --create-namespace \
  --set rbac.viewRole.enabled=true

# Or grant a scoped, custom read-only rule (preferred for production):
helm install kube-drift ./helm/kube-drift \
  --namespace kube-drift-system --create-namespace \
  --set-json 'rbac.extraRules=[{"apiGroups":["apps"],"resources":["deployments"],"verbs":["get","list","watch"]}]'
```

**Kustomize install (fallback).** The chart knobs are not available on the `make deploy` path; grant access manually. The simplest grant is the built-in `view` ClusterRole:

```bash
kubectl create clusterrolebinding kube-drift-view \
  --clusterrole=view \
  --serviceaccount=kube-drift-system:kube-drift-controller-manager
```

Scope this down to specific kinds with a purpose-built ClusterRole for production.

<br/>

## Notifications

Add `spec.notify.webhooks` to receive a message whenever the **drift state changes** — either drift is newly detected, the set of drifted resources changes, or drift is resolved. Notifications are **deduplicated**: a message is sent only when the drifted set changes (fingerprinted in `status.lastNotifiedHash`), not on every `interval` re-check.

```yaml
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: driftcheck-sample
  namespace: default
spec:
  source:
    type: ConfigMap
    configMap:
      name: desired-manifests
  interval: 5m
  notify:
    webhooks:
      - type: Slack           # posts {"text": "..."} to a Slack incoming webhook
        urlSecretRef:          # prefer a Secret over an inline URL
          name: slack-webhook
          key: url
      - type: Generic         # posts a structured JSON body
        url: http://alertmanager-webhook.monitoring.svc/drift
```

Create the Slack webhook Secret in the DriftCheck's namespace:

```bash
kubectl -n default create secret generic slack-webhook \
  --from-literal=url='https://hooks.slack.com/services/XXX/YYY/ZZZ'
```

Each webhook has a `type`:

- **`Slack`** — a human-readable `{"text": "..."}` message listing the drifted resources. Point it at a Slack [incoming webhook](https://api.slack.com/messaging/webhooks).
- **`Generic`** (default) — a structured JSON body: `{driftCheck, namespace, resolved, summary, drifted[]}`. Point it at Alertmanager's webhook receiver or any HTTP endpoint.

The URL comes from either an inline `url` or a `urlSecretRef` (a `Secret` in the DriftCheck's namespace); `urlSecretRef` takes precedence and is recommended for Slack URLs. Sourcing the URL from a Secret is why the controller declares `secrets: get` RBAC.

Delivery is **best-effort with at-least-once** semantics: a webhook that fails is logged and surfaced as a `NotifyFailed` event (`kubectl describe driftcheck <name>`), and the notification is retried on the next reconcile — a retry re-sends to every configured webhook.

<br/>

## Troubleshooting

If a DriftCheck never reports drift, inspect its `Ready` condition:

```bash
kubectl get driftcheck <name> -o jsonpath='{.status.conditions}' | jq
```

| `Ready` reason | Meaning | Fix |
|---|---|---|
| `DriftEvaluated` (True) | The check ran successfully | — |
| `SourceError` (False) | Desired manifests could not be loaded — missing ConfigMap, bad key, missing `git` block, or empty `url` | Correct `spec.source` |
| `NoFetcher` (False) | The cluster fetcher was not wired (controller misconfiguration) | Check the controller logs |
| `CompareError` (False) | A transient comparison failure — API blip or a Git clone that timed out | Usually self-heals on backoff retry; check network/repo reachability |

Controller logs:

```bash
kubectl -n kube-drift-system logs deploy/kube-drift-controller-manager -f
```

<br/>

## Limitations

- **Git private repos via `auth` only** — the top-level Git / Helm / Kustomize clone supports private repositories through `source.git.auth` (Basic / Bearer / SSH); without `auth` the clone is anonymous. Remote Kustomize bases and Helm dependencies fetched via `dependencyBuild` are still retrieved anonymously.
- **Self-contained Helm charts by default** — chart dependencies are expected to be vendored under `charts/`. Charts with external dependencies can opt into `source.helm.dependencyBuild: true`, which fetches them at render time over the network (HTTP(S) or `oci://` repositories only; named `@alias` repos are unsupported).
- **ConfigMap read RBAC by default** — comparing other kinds requires granting read access, via the chart's `rbac.viewRole` / `rbac.extraRules` knobs (Helm) or a manual ClusterRoleBinding (Kustomize) (see [RBAC](#rbac-for-non-configmap-kinds)).
