# Changelog

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
