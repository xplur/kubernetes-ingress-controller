#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

mkdir -pv deploy/single-v1 deploy/single-v2
ln -sfnv single-v2 deploy/single

# k4k8s
kustomize build ./deploy/manifests/base-v1 > deploy/single-v1/all-in-one-dbless.yaml
# k4k8s with DB
kustomize build ./deploy/manifests/postgres \
  > deploy/single-v1/all-in-one-postgres.yaml
# k4k8s Enterprise
kustomize build ./deploy/manifests/enterprise-k8s \
  > deploy/single-v1/all-in-one-dbless-k4k8s-enterprise.yaml
# Kong Enterprise
kustomize build ./deploy/manifests/enterprise \
  > deploy/single-v1/all-in-one-postgres-enterprise.yaml

# Kong Dev Config
cat ./deploy/manifests/base-v1/custom-types.yaml \
  > hack/dev/common/custom-types.yaml
