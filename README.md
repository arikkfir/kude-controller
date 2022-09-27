# kude-controller

![Maintainer](https://img.shields.io/badge/maintainer-arikkfir-blue)
![GoVersion](https://img.shields.io/github/go-mod/go-version/arikkfir/kude-controller.svg)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/arikkfir/kude-controller)
[![GoReportCard](https://goreportcard.com/badge/github.com/arikkfir/kude-controller)](https://goreportcard.com/report/github.com/arikkfir/kude-controller)
[![codecov](https://codecov.io/gh/arikkfir/kude-controller/branch/main/graph/badge.svg?token=QP3OAILB25)](https://codecov.io/gh/arikkfir/kude-controller)

## Development

### Setup

Assumptions:
- Git is available as `git`
- Go is available as `go` and `GOPATH` is set to outside of the project directory (not a parent of it!)
- The `$GOPATH/bin` directory is part of your `$PATH` environment variable.

```shell
$ git clone https://github.com/arikkfir/kude-controller.git
$ cd kude-controller
$ go mod download
$ go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2
$ go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
$ setup-envtest use 1.25.0 --bin-dir=./bin
```

### Run

```shell
$ skaffold dev
```

### Generate code

```shell
$ controller-gen object paths="./internal/v1alpha1"
$ controller-gen rbac:roleName=kude-controller crd webhook paths="./..."
```

### ROADMAP

- [ ] Upgrade to Gingo v2
- [ ] Implement Helm bundle
- [ ] Implement Kude bundle
- [ ] Implement Kustomize bundle
- [ ] Setup distribution methods of Kude Controller (e.g. Helm, Kustomize, YAML, etc)
