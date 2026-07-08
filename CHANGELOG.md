# Changelog

All notable changes to this project will be documented in this file.

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

