<!--
SPDX-FileCopyrightText: 2023 - 2025 NRK

SPDX-License-Identifier: MIT
-->

# Changelog

## [0.21.0](https://github.com/nrkno/terraform-registry/compare/v0.20.1...v0.21.0) (2025-10-29)


### Features

* support authenticating as github app ([5183790](https://github.com/nrkno/terraform-registry/commit/51837900e73f456b54391160d00bb1157c474659))

## [0.20.1](https://github.com/nrkno/terraform-registry/compare/v0.20.0...v0.20.1) (2025-01-13)


### Bug Fixes

* don't update cache if ratelimited ([63d87a0](https://github.com/nrkno/terraform-registry/commit/63d87a0621cd4711c34b6a16622c3b45c466ecf4))
* remove debug log for unchanged watchFile ([e70d874](https://github.com/nrkno/terraform-registry/commit/e70d8749c3b7004ad77f46dc7b38731ebb6fd683))
* **store/github:** improve details in empty provider result warning ([cf733fb](https://github.com/nrkno/terraform-registry/commit/cf733fb033b49798b690e551cd726cf3d3a1ff23))
* **store/github:** load module cache before providers ([a36dbbc](https://github.com/nrkno/terraform-registry/commit/a36dbbc1032a9fa1c8da81169989a83bc82f2844))
* **store/github:** log warning when no module repos were found ([b70f72f](https://github.com/nrkno/terraform-registry/commit/b70f72f5d505230fd50ecda039bdf0a6a6a53ecf))

## [0.20.0](https://github.com/nrkno/terraform-registry/compare/v0.19.0...v0.20.0) (2024-04-17)


### Features

* add v1/providers protocol ([2a7295c](https://github.com/nrkno/terraform-registry/commit/2a7295c4c741eb6b3a90d8d69cde35b7effb2368))


### Bug Fixes

* check if provider version exists in cache before trying to download ([9b1b0b7](https://github.com/nrkno/terraform-registry/commit/9b1b0b7a933b866cc0c83c1ae2ddb6871bb62229))
* ignore releases previously found not valid ([c9a3368](https://github.com/nrkno/terraform-registry/commit/c9a336837c3f606e66d2babe208526a7ce4705fd))
* proxy signature url ([c2aa0db](https://github.com/nrkno/terraform-registry/commit/c2aa0db0ce5c3e01a733c05e726ef6f8b7c02aa3))

## [0.19.0](https://github.com/nrkno/terraform-registry/compare/v0.18.0...v0.19.0) (2024-01-17)


### ⚠ BREAKING CHANGES

* remove redundant log on empty auth token file
* require -auth-disabled when -auth-tokens-file is unset

### Bug Fixes

* remove redundant log on empty auth token file ([03bc360](https://github.com/nrkno/terraform-registry/commit/03bc3605f7dc3c8e7c8504ded0a234a93a1a5fdc))
* require -auth-disabled when -auth-tokens-file is unset ([b632295](https://github.com/nrkno/terraform-registry/commit/b6322953ec46d6018cfb17ff31f0feae35cad3f7))

## [0.18.0](https://github.com/nrkno/terraform-registry/compare/v0.17.0...v0.18.0) (2024-01-11)


### Features

* add -version arg ([3d16771](https://github.com/nrkno/terraform-registry/commit/3d1677151209ddb565dbe71bb3626e950065290a))


### Bug Fixes

* avoid race when verifying auth tokens ([6fe703a](https://github.com/nrkno/terraform-registry/commit/6fe703afcf62da0e4751d8f9fc1062084da6a3af))

## [0.17.0](https://github.com/nrkno/terraform-registry/compare/v0.16.0...v0.17.0) (2023-12-20)


### Features

* Add support for s3 as a backend store ([3619ebe](https://github.com/nrkno/terraform-registry/commit/3619ebe23f61442d6695002748bacd46aa89f1c1))

## [0.16.0](https://github.com/nrkno/terraform-registry/compare/v0.15.0...v0.16.0) (2023-12-19)


### Features

* allow disabling HTTP access log ([dafa222](https://github.com/nrkno/terraform-registry/commit/dafa222aa39964ae676a3e0f4a89b13d2833c2fe)), closes [#63](https://github.com/nrkno/terraform-registry/issues/63)
* allow ignoring certain paths from access log ([7c48d2f](https://github.com/nrkno/terraform-registry/commit/7c48d2f2f7587de41cbdbb6a43b992c2fbcd4c4b))
* use zap for http access logging ([df72911](https://github.com/nrkno/terraform-registry/commit/df7291144fd4972198ddb134afe1cdb8c5661598)), closes [#54](https://github.com/nrkno/terraform-registry/issues/54)

## [0.15.0](https://github.com/nrkno/terraform-registry/compare/v0.14.0...v0.15.0) (2023-03-17)


### ⚠ BREAKING CHANGES

* do not crash if reading auth tokens fail
* store registry auth tokens as a map
* remove support for newline separated auth file

### Features

* automatically update auth tokens on auth file changes ([e3d406e](https://github.com/nrkno/terraform-registry/commit/e3d406e632e2254001027afd5f57da240706ab91))
* remove support for newline separated auth file ([8fce7d7](https://github.com/nrkno/terraform-registry/commit/8fce7d7d659c5c812e983618f42285d4898d39c2))
* store registry auth tokens as a map ([6294753](https://github.com/nrkno/terraform-registry/commit/629475355c998e15a13b6bc722e3cd2ef166aa15))


### Bug Fixes

* do not crash if reading auth tokens fail ([40354e6](https://github.com/nrkno/terraform-registry/commit/40354e66bec988c4724d937a79dcdab348e76066))
* remove unimplemented login service definition ([b9e0ff0](https://github.com/nrkno/terraform-registry/commit/b9e0ff06207d39f76ff0bfe021d4c5de92fe9e9e))

## 0.14.0 (2022-09-20)


### ⚠ BREAKING CHANGES

* warn instead of panic for empty auth tokens file
* only apply auth to /v1 API endpoints
* set main binary as entrypoint in Dockerfile
* rename github cmd args to better reflect usage

### Features

* add build-docker target to makefile ([2e9bb34](https://github.com/nrkno/terraform-registry/commit/2e9bb34cf47899508f0d4fceff28c0698af1181e))
* add initial fuzz testing of auth ([5eb4c95](https://github.com/nrkno/terraform-registry/commit/5eb4c956486cf1b23c99c5d37ddff67852cff365))
* add log level and format arguments ([8cadf9a](https://github.com/nrkno/terraform-registry/commit/8cadf9afc931571eb3407792376d3d15d8ed3571))
* add routes fuzz test ([ef30a17](https://github.com/nrkno/terraform-registry/commit/ef30a179b5a9a19d8a4775f82c71df15c99c35ae))
* add support for auth token file to be a json file ([#26](https://github.com/nrkno/terraform-registry/issues/26)) ([ccbb94a](https://github.com/nrkno/terraform-registry/commit/ccbb94a04ef4500249fc90ae468437bb4af6d3cd))
* check github tags for valid semver version ([e3c1375](https://github.com/nrkno/terraform-registry/commit/e3c1375002aa33f6ee75306b995fb26e8b6773cf))
* more acurate handling of .well-known to avoid making the . in terraform.json regexp any ([858ba28](https://github.com/nrkno/terraform-registry/commit/858ba282edae7dcb8047665946c4a8a0dddc2b69))
* only apply auth to /v1 API endpoints ([b3e0521](https://github.com/nrkno/terraform-registry/commit/b3e0521c33dc0ea7844670b24e1f26f0718554fa))
* return JSON from /health endpoint ([66d160f](https://github.com/nrkno/terraform-registry/commit/66d160f4dd6d9e6b2c3b3d96ec9554cfa8f089e5))
* safeguard index handler, simplify test logic ([c3a4e86](https://github.com/nrkno/terraform-registry/commit/c3a4e865ba55523ac0152279a00882803ba76f12))
* show help text when started without args ([#23](https://github.com/nrkno/terraform-registry/issues/23)) ([e41da8a](https://github.com/nrkno/terraform-registry/commit/e41da8a940e0f926fbc75900809b6a40382c1485))
* **store/github:** require only at least one filter ([#24](https://github.com/nrkno/terraform-registry/issues/24)) ([771f335](https://github.com/nrkno/terraform-registry/commit/771f335a20041043d26ba937b623dcfb7a7dbfbf))
* support parsing and setting env from JSON env files ([686eddc](https://github.com/nrkno/terraform-registry/commit/686eddc426a73c100ed826baa32992224e3a992f))
* use a leveled structured log library ([59b6170](https://github.com/nrkno/terraform-registry/commit/59b61709cfba20b43f21c42a056b92b2234f1cc2))


### Bug Fixes

* avoid panic when no -env-json-files are specified ([3b972e2](https://github.com/nrkno/terraform-registry/commit/3b972e217578188f4684d1ff27b7a3a3b53f478f))
* handle odd cases for /v1 parsing ([7966819](https://github.com/nrkno/terraform-registry/commit/7966819c04ee157a33fa1fd775ea34e5eceb9c03))
* return known http error strings on NotFound and MethodNotAllowed ([b5509a9](https://github.com/nrkno/terraform-registry/commit/b5509a9dd1b59717020835ed258871632967b583))
* set main binary as entrypoint in Dockerfile ([70b5e60](https://github.com/nrkno/terraform-registry/commit/70b5e6011d91c827df5714bbc8b1ad5fc0d57c29))
* **store/github:** error instead of panic on initial cache load error ([f1887eb](https://github.com/nrkno/terraform-registry/commit/f1887eb0312baeb9dd825d8c8efc28f2004fe14d))
* use Makefile in Dockerfile ([f8feb73](https://github.com/nrkno/terraform-registry/commit/f8feb732da67f30a38fe48a336e0b31e92624789)), closes [#8](https://github.com/nrkno/terraform-registry/issues/8)
* warn instead of panic for empty auth tokens file ([5eab4c2](https://github.com/nrkno/terraform-registry/commit/5eab4c2edcbfbc5c58a2f3b26208dfff55418268))


### Miscellaneous Chores

* rename github cmd args to better reflect usage ([1d7bdd3](https://github.com/nrkno/terraform-registry/commit/1d7bdd3563e4a800d944f0dddf8c76822b745041))
