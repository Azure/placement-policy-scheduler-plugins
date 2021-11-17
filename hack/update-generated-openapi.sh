#!/usr/bin/env bash
#https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/hack/update-generated-openapi.sh

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[@]}")/..


TOOLS_DIR=$(realpath ./hack/tools)
TOOLS_BIN_DIR="${TOOLS_DIR}/bin"
GO_INSTALL=$(realpath ./hack/go-install.sh)


pushd "${SCRIPT_ROOT}"
  # install the openapi-gen if they are not already present
  GOBIN=${TOOLS_BIN_DIR} ${GO_INSTALL} k8s.io/code-generator/cmd/openapi-gen openapi-gen v0.22.3
popd

KUBE_INPUT_DIRS=(
  $(
    grep --color=never -rl '+k8s:openapi-gen=' vendor/k8s.io | \
    xargs -n1 dirname | \
    sed "s,^vendor/,," | \
    sort -u | \
    sed '/^k8s\.io\/kubernetes\/build\/root$/d' | \
    sed '/^k8s\.io\/kubernetes$/d' | \
    sed '/^k8s\.io\/kubernetes\/staging$/d' | \
    sed 's,k8s\.io/kubernetes/staging/src/,,' | \
    grep -v 'k8s.io/code-generator' | \
    grep -v 'k8s.io/sample-apiserver'
  )
)

KUBE_INPUT_DIRS=$(IFS=,; echo "${KUBE_INPUT_DIRS[*]}")

function join { local IFS="$1"; shift; echo "$*"; }

echo "Generating Kubernetes OpenAPI"

"${TOOLS_BIN_DIR}/openapi-gen" \
  --output-file-base zz_generated.openapi \
  --output-base="${TOOLS_BIN_DIR}/src" \
  --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.go.txt \
  --output-base="./" \
  --input-dirs $(join , "${KUBE_INPUT_DIRS[@]}") \
  --output-package "vendor/k8s.io/kubernetes/pkg/generated/openapi" \
  "$@"
