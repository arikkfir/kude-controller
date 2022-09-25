#!/usr/bin/env bash

set -exuo pipefail

set +e
kustomize build test/deploy --reorder=none | kubectl create -f -
set -e

exit 0
