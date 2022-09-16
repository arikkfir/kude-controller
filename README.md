# kude-controller

## Development

### Setup

```shell
$ git clone https://github.com/arikkfir/kude-controller.git
$ cd kude-controller
$ go mod download
$ go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2
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
