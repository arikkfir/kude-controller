#!/usr/bin/env bash

set -exuo pipefail

set +e
for rt in GitRepository KubectlBundle; do
  kubectl get --namespace kude ${rt} --output=name \
    | xargs -I@ kubectl patch --namespace kude @ --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
done
kustomize build test/deploy --reorder=none | kubectl delete --filename=-
kubectl delete namespace kubectl-bundle
set -e

exit 0
