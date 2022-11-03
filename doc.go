//go:generate controller-gen object rbac:roleName=kude-controller crd webhook paths="./..." output:rbac:artifacts:config=chart/templates output:crd:artifacts:config=chart/crds
package kude_controller
