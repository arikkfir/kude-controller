#!/usr/bin/env bash

set -exuo pipefail

set +e
for rt in GitRepository KubectlBundle KubectlRun; do
  kubectl get --namespace kude ${rt} --output=name \
    | xargs -I@ kubectl patch --namespace kude @ --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
done
#kubectl patch --namespace kude GitRepository --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
#kubectl patch --namespace kude KubectlBundle --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
#kubectl patch --namespace kude KubectlRun --type=json '--patch=[{"op":"remove","path":"/metadata/finalizers"}]'
kustomize build test/deploy --reorder=none | kubectl delete --filename=-
set -e

exit 0
