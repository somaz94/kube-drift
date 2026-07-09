# Changelog

All notable changes to this project will be documented in this file.

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

