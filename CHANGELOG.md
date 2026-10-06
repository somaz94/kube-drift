# Changelog

All notable changes to this project will be documented in this file.

## [v0.5.0](https://github.com/somaz94/kube-drift/compare/v0.4.0...v0.5.0) (2026-10-06)

### Features

- filter desired manifests by spec.target namespaces and label selector ([d27cc54](https://github.com/somaz94/kube-drift/commit/d27cc54bd5b9ab0a94e2a9e60cdade38c95fa467))

### Bug Fixes

- bump kube-diff to v0.6.0 to resolve kinds through API discovery ([42fd1c2](https://github.com/somaz94/kube-drift/commit/42fd1c2604bc9aeacf083a2170c3c52e4484a048))
- bump kube-diff to v0.5.4 and report forbidden reads as FetchError ([939e0b0](https://github.com/somaz94/kube-drift/commit/939e0b0eec55d03f1119d84470bb4300089ab7e9))
- strip a trailing newline from the SSH key passphrase ([d40a795](https://github.com/somaz94/kube-drift/commit/d40a795e333d8372d6aa7f366b473129b1e57774))
- exclude e2e tests from make pr ([de50436](https://github.com/somaz94/kube-drift/commit/de50436b891b937d5eb5b29ed0f35963e091a807))
- skip Helm subcharts disabled by condition or tags ([cb2051f](https://github.com/somaz94/kube-drift/commit/cb2051fdbed6309b2b88e104550aaad16122246e))
- add the metrics bind address without replacing manager args ([a9da3dd](https://github.com/somaz94/kube-drift/commit/a9da3dd6580dd26384dfdc2501f14f9be7f0148e))
- reject an empty ConfigMap entry selected by key ([e8dcb5f](https://github.com/somaz94/kube-drift/commit/e8dcb5f9e7e05ebdd035ed90cb3b0a5d503a9640))
- resolve oci:// dependency version ranges without crashing the manager ([54c7915](https://github.com/somaz94/kube-drift/commit/54c7915e9cb0ad76883da89acb4b6366bf439ba5))
- **chart:** declare the Kubernetes floor the CRD's CEL validation rules require ([b3e4b62](https://github.com/somaz94/kube-drift/commit/b3e4b6217e9ba43909872a9787b7a1ca58341773))

### Code Refactoring

- name repeated string literals as constants ([4560f5e](https://github.com/somaz94/kube-drift/commit/4560f5e7a2450c60dd45dba8fa2405ce542c9f6d))
- preallocate drifted slice with results capacity ([164f010](https://github.com/somaz94/kube-drift/commit/164f01032c168224050b65e4a5c9509e449f8a35))

### Documentation

- record scoped comparison and fetch-error handling in the roadmap ([c03b00c](https://github.com/somaz94/kube-drift/commit/c03b00c42e9c251356e460505af2c8553b618c6b))
- correct kustomize remote base, load restriction, and deleted status notes ([ee34908](https://github.com/somaz94/kube-drift/commit/ee349086fbcceb1c44865bbac74720ad81ee58a1))
- document the kubectl apply install path and uninstall steps ([701a777](https://github.com/somaz94/kube-drift/commit/701a777aeb3a41ec2c367c1a81ebe2652965629b))
- state the Kubernetes v1.25+ requirement in the prerequisites ([d521232](https://github.com/somaz94/kube-drift/commit/d521232a27995ce64628e271372c94f30c69d2dc))

### Builds

- **deps:** bump the go-minor group with 5 updates (#15) ([#15](https://github.com/somaz94/kube-drift/pull/15)) ([e49b77c](https://github.com/somaz94/kube-drift/commit/e49b77c90647da264e6f335560c2a5deb45860ec))
- **deps:** bump the go-minor group with 3 updates (#14) ([#14](https://github.com/somaz94/kube-drift/pull/14)) ([734b73b](https://github.com/somaz94/kube-drift/commit/734b73b544f97d5ae0481052e5af76d91a4ec0cb))
- **deps:** bump the go-minor group with 3 updates (#13) ([#13](https://github.com/somaz94/kube-drift/pull/13)) ([1ca53c6](https://github.com/somaz94/kube-drift/commit/1ca53c6148d0c1f4080a3e98f1e6e2f708ae413b))
- **deps:** bump the go-minor group with 2 updates (#12) ([#12](https://github.com/somaz94/kube-drift/pull/12)) ([b8443d3](https://github.com/somaz94/kube-drift/commit/b8443d35dd5aefba3a4c703234c5d9c48df4cc28))
- **deps:** bump the go-minor group with 4 updates (#11) ([#11](https://github.com/somaz94/kube-drift/pull/11)) ([1ac0ce5](https://github.com/somaz94/kube-drift/commit/1ac0ce5cb13e328528565fbf810959a4cf33d148))
- **deps:** bump golang from 1.26 to 1.27 in the docker-minor group (#10) ([#10](https://github.com/somaz94/kube-drift/pull/10)) ([0cde3aa](https://github.com/somaz94/kube-drift/commit/0cde3aaaf1c146a534b6efff067d6ed02947e1f2))
- **deps:** bump the go-minor group with 4 updates (#9) ([#9](https://github.com/somaz94/kube-drift/pull/9)) ([f014ef1](https://github.com/somaz94/kube-drift/commit/f014ef1bc3baa95bb2690d14e9b71c81cf64e131))
- **deps:** bump the go-minor group with 4 updates (#8) ([#8](https://github.com/somaz94/kube-drift/pull/8)) ([2f72e8f](https://github.com/somaz94/kube-drift/commit/2f72e8fca58262af7b3114f1e48421114b7456d0))
- **deps:** bump actions/stale from 10 to 11 (#5) ([#5](https://github.com/somaz94/kube-drift/pull/5)) ([dbeff56](https://github.com/somaz94/kube-drift/commit/dbeff56e03741ba7c91c309bf28ac14b0ee01e11))
- **deps:** bump github.com/somaz94/kube-diff in the go-minor group (#7) ([#7](https://github.com/somaz94/kube-drift/pull/7)) ([435a745](https://github.com/somaz94/kube-drift/commit/435a7451905902a445afc1342cda51dac8b96ea2))
- **deps:** bump github.com/go-git/go-git/v5 in the go-minor group (#6) ([#6](https://github.com/somaz94/kube-drift/pull/6)) ([2d829fd](https://github.com/somaz94/kube-drift/commit/2d829fdc93dd3b592ff5a95f13de7ec4d6a6aee8))
- **deps:** bump the go-minor group with 5 updates (#4) ([#4](https://github.com/somaz94/kube-drift/pull/4)) ([7ee88da](https://github.com/somaz94/kube-drift/commit/7ee88da4df2afad2b06ab6038b7a5ae9cb672a69))
- **deps:** bump actions/setup-go from 6 to 7 (#3) ([#3](https://github.com/somaz94/kube-drift/pull/3)) ([fc11af9](https://github.com/somaz94/kube-drift/commit/fc11af989c92e38a4244de36346b0bc0699caad1))
- **deps:** bump the go-minor group with 2 updates (#2) ([#2](https://github.com/somaz94/kube-drift/pull/2)) ([961eae4](https://github.com/somaz94/kube-drift/commit/961eae4400b1535eafeff5a0fe25890531d97158))
- **deps:** bump the go-minor group with 5 updates (#1) ([#1](https://github.com/somaz94/kube-drift/pull/1)) ([53ca109](https://github.com/somaz94/kube-drift/commit/53ca109c354f50efc4e925d34bdafe25ee32c185))

### Continuous Integration

- surface the generated-code error and check the Helm CRD copy ([cdae3f9](https://github.com/somaz94/kube-drift/commit/cdae3f92c1e92c84a58740b3626a564bbbd0d852))
- trim redundant comments in gitlab-mirror workflow ([ebb6184](https://github.com/somaz94/kube-drift/commit/ebb6184a719a2c3573e537555be089e25ad0a88f))
- retry mirror pushes on transient remote failures ([37b4cfa](https://github.com/somaz94/kube-drift/commit/37b4cfaf4aa1ef20f1f64ab6496532d270d3b422))
- drop the dead issue-close trigger from changelog generation ([de44020](https://github.com/somaz94/kube-drift/commit/de44020dc12d293425c6f0bbcc90ecbaac4074ce))
- remove DCO workflow ([b910060](https://github.com/somaz94/kube-drift/commit/b910060794d9cf3682b98cc0bc7292f9fd246773))

### Chores

- bump version to v0.5.0 ([ecb26be](https://github.com/somaz94/kube-drift/commit/ecb26beb3e03668fac906596767d149fc15dc849))
- fix stale help text, chart notes, image label, and ignore rules ([871ba33](https://github.com/somaz94/kube-drift/commit/871ba335f5a330c3ad5234f47ee9755d7b6e75bf))
- correct stale and redundant comments in config and helm chart ([c61c56a](https://github.com/somaz94/kube-drift/commit/c61c56abf05e5ccbd7fe6a1d27132d64ccb180fa))
- correct stale and redundant comments in controller, metrics, notify, and cmd ([1bed9b2](https://github.com/somaz94/kube-drift/commit/1bed9b299ee94455d7ec68ec74c77175c0398c6f))
- correct stale and redundant comments in internal/source ([19ffcde](https://github.com/somaz94/kube-drift/commit/19ffcdecbff5d2841e0646b92cba6410cd9581f4))
- trim stale and redundant comments in build, CI, and e2e files ([c235cde](https://github.com/somaz94/kube-drift/commit/c235cde9bf0f9c95c9370e80d351249589506c7f))
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

