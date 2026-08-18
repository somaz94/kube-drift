# Changelog

All notable changes to this project will be documented in this file.

## Unreleased (2026-08-18)

### Bug Fixes

- **chart:** declare the Kubernetes floor the CRD's CEL validation rules require ([b3e4b62](https://github.com/somaz94/kube-drift/commit/b3e4b6217e9ba43909872a9787b7a1ca58341773))

### Code Refactoring

- name repeated string literals as constants ([4560f5e](https://github.com/somaz94/kube-drift/commit/4560f5e7a2450c60dd45dba8fa2405ce542c9f6d))
- preallocate drifted slice with results capacity ([164f010](https://github.com/somaz94/kube-drift/commit/164f01032c168224050b65e4a5c9509e449f8a35))

### Documentation

- document the kubectl apply install path and uninstall steps ([701a777](https://github.com/somaz94/kube-drift/commit/701a777aeb3a41ec2c367c1a81ebe2652965629b))
- state the Kubernetes v1.25+ requirement in the prerequisites ([d521232](https://github.com/somaz94/kube-drift/commit/d521232a27995ce64628e271372c94f30c69d2dc))

### Builds

- **deps:** bump actions/stale from 10 to 11 (#5) ([#5](https://github.com/somaz94/kube-drift/pull/5)) ([dbeff56](https://github.com/somaz94/kube-drift/commit/dbeff56e03741ba7c91c309bf28ac14b0ee01e11))
- **deps:** bump github.com/somaz94/kube-diff in the go-minor group (#7) ([#7](https://github.com/somaz94/kube-drift/pull/7)) ([435a745](https://github.com/somaz94/kube-drift/commit/435a7451905902a445afc1342cda51dac8b96ea2))
- **deps:** bump github.com/go-git/go-git/v5 in the go-minor group (#6) ([#6](https://github.com/somaz94/kube-drift/pull/6)) ([2d829fd](https://github.com/somaz94/kube-drift/commit/2d829fdc93dd3b592ff5a95f13de7ec4d6a6aee8))
- **deps:** bump the go-minor group with 5 updates (#4) ([#4](https://github.com/somaz94/kube-drift/pull/4)) ([7ee88da](https://github.com/somaz94/kube-drift/commit/7ee88da4df2afad2b06ab6038b7a5ae9cb672a69))
- **deps:** bump actions/setup-go from 6 to 7 (#3) ([#3](https://github.com/somaz94/kube-drift/pull/3)) ([fc11af9](https://github.com/somaz94/kube-drift/commit/fc11af989c92e38a4244de36346b0bc0699caad1))
- **deps:** bump the go-minor group with 2 updates (#2) ([#2](https://github.com/somaz94/kube-drift/pull/2)) ([961eae4](https://github.com/somaz94/kube-drift/commit/961eae4400b1535eafeff5a0fe25890531d97158))
- **deps:** bump the go-minor group with 5 updates (#1) ([#1](https://github.com/somaz94/kube-drift/pull/1)) ([53ca109](https://github.com/somaz94/kube-drift/commit/53ca109c354f50efc4e925d34bdafe25ee32c185))

### Continuous Integration

- remove DCO workflow ([b910060](https://github.com/somaz94/kube-drift/commit/b910060794d9cf3682b98cc0bc7292f9fd246773))

### Chores

- track dist/install.yaml install manifest ([0944c69](https://github.com/somaz94/kube-drift/commit/0944c69e9b65fea4b404a0694931c2fd5994e8ce))
- extract repeated test literals into constants ([3e24994](https://github.com/somaz94/kube-drift/commit/3e249945a75db6a4d6f42979a5cbd5d7513f4da6))
- resolve golangci-lint errcheck and staticcheck findings ([778266e](https://github.com/somaz94/kube-drift/commit/778266e8efeb721c5049a030929c8b006150edca))
- **lint:** use the golangci-lint v2 module path and config schema ([5873110](https://github.com/somaz94/kube-drift/commit/5873110c54f35bc2843be59b9a8313374a8ecd2b))

### Contributors

- somaz

<br/>

## [v0.4.0](https://github.com/somaz94/kube-drift/compare/v0.3.0...v0.4.0) (2026-07-09)

### Features

- add opt-in in-process Helm dependency build ([39b0344](https://github.com/somaz94/kube-drift/commit/39b03448c409a0a0df7f6ed96b06df98ace295bd))
- add opt-in broad read-RBAC chart knobs (viewRole, extraRules) ([7f24a12](https://github.com/somaz94/kube-drift/commit/7f24a123b09316daba8fcb2315e5f00adb9715e3))

### Chores

- bump version to v0.4.0 ([8c8ddff](https://github.com/somaz94/kube-drift/commit/8c8ddff81d312cf60605fc1f60a1e187de691f7c))

### Contributors

- somaz

<br/>

## [v0.3.0](https://github.com/somaz94/kube-drift/compare/v0.2.0...v0.3.0) (2026-07-09)

### Features

- add Git credential support for private repository clones ([3c33cdc](https://github.com/somaz94/kube-drift/commit/3c33cdc17b6947f478954931f2ca89c86b7de6c9))

### Tests

- add e2e scenarios for Helm/Kustomize sources and notifications ([5cbc7b9](https://github.com/somaz94/kube-drift/commit/5cbc7b93acaf888154f704ba686ef904f37bc45f))

### Continuous Integration

- cross-compile multi-arch image to avoid slow arm64 emulation ([c7d5a24](https://github.com/somaz94/kube-drift/commit/c7d5a244f60ec2bad8c995f28b6765c4441fa521))

### Chores

- bump version to v0.3.0 ([4898a5d](https://github.com/somaz94/kube-drift/commit/4898a5dd59191e8a7c7f8aa5d0bfdbbaceec6456))

### Contributors

- somaz

<br/>

## [v0.2.0](https://github.com/somaz94/kube-drift/compare/v0.1.0...v0.2.0) (2026-07-09)

### Features

- add in-process Helm and Kustomize sources ([641e2fb](https://github.com/somaz94/kube-drift/commit/641e2fb447c37157ee00e91f5e3dca96d6cf8c04))
- add Slack/webhook drift notifications ([89fbff0](https://github.com/somaz94/kube-drift/commit/89fbff05292fd8f1d9f13cde6a4611d2006e45e2))
- add helm controller templates (deployment, rbac, service, sa) ([0d7fbed](https://github.com/somaz94/kube-drift/commit/0d7fbed3ff947f12ad558ac6cb610d8872d06f8d))

### Continuous Integration

- build and push docker image on release tag ([b118f53](https://github.com/somaz94/kube-drift/commit/b118f53401fb0fc153d424371a5626e5a9b05973))

### Chores

- bump version to v0.2.0 ([663b0f9](https://github.com/somaz94/kube-drift/commit/663b0f9c91ebc05184abe62876c76375c0a263b4))

### Contributors

- somaz

<br/>

## [v0.1.0](https://github.com/somaz94/kube-drift/releases/tag/v0.1.0) (2026-07-08)

### Features

- add Git desired-state source with go-git cloner ([523df31](https://github.com/somaz94/kube-drift/commit/523df3194a9181ba9227a177e0deb3af2300423c))
- expose kube_drift_resources drift gauge metric ([65bc8b9](https://github.com/somaz94/kube-drift/commit/65bc8b9d6156604eeab29a74f77ed84c04781631))
- detect ConfigMap-source drift via kube-diff engine ([4a2ed82](https://github.com/somaz94/kube-drift/commit/4a2ed826cf91834a6d20e8e64695885a53f8c1b3))
- scaffold kube-drift operator with DriftCheck CRD ([e14395d](https://github.com/somaz94/kube-drift/commit/e14395df57c40daceeb71f2befd7fc7cf4c93897))

### Documentation

- add USAGE guide and link it from README ([5b849e5](https://github.com/somaz94/kube-drift/commit/5b849e5f7156f5c893599be52ce866f6a18b1d11))
- describe kube-drift operator and DriftCheck CRD ([0d8f7a1](https://github.com/somaz94/kube-drift/commit/0d8f7a148187b0b981aca6df83fa72dd6310e074))

### Tests

- restore kind e2e suite with metrics RBAC and drift scenario ([68641ee](https://github.com/somaz94/kube-drift/commit/68641eed58181c319c97cf54f0b0fc85d7036a61))

### Builds

- depend on published kube-diff v0.5.0 ([329238b](https://github.com/somaz94/kube-drift/commit/329238b9c175c284c3a4bc4ba7f06df2d265992f))

### Continuous Integration

- point cliff.toml at the kube-drift repo ([90326b7](https://github.com/somaz94/kube-drift/commit/90326b791598511203567d7b3f45c6abc0f46bcc))
- gate e2e workflow on manual dispatch until Phase 2 ([95eff9b](https://github.com/somaz94/kube-drift/commit/95eff9b64ee16a286be5bb4a50bad4f66391fbae))

### Contributors

- somaz

<br/>

