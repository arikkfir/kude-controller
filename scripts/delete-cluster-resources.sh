#!/usr/bin/env bash

set -exuo pipefail

set +e
kubectl patch --namespace kude gitrepository kude-controller-main --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
set -e
