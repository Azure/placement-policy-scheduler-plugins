#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[@]}")/..

TOOLS_DIR=$(realpath ./hack/tools)
TOOLS_BIN_DIR="${TOOLS_DIR}/bin"
GO_INSTALL=$(realpath ./hack/go-install.sh)

pushd "${SCRIPT_ROOT}"
# install the generators if they are not already present
for GENERATOR in client-gen lister-gen informer-gen register-gen; do
  GOBIN=${TOOLS_BIN_DIR} ${GO_INSTALL} k8s.io/code-generator/cmd/${GENERATOR} ${GENERATOR} v0.22.3
done
popd

OUTPUT_PKG=github.com/Azure/placement-policy-scheduler-plugins/pkg/client
FQ_APIS=github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1
CLIENTSET_NAME=versioned
CLIENTSET_PKG_NAME=clientset

# reference from https://github.com/servicemeshinterface/smi-sdk-go/blob/master/hack/update-codegen.sh
# the generate-groups.sh script cannot handle group names with dashes, so we use placementpolicy.scheduling.x-k8s.io as the group name
if [[ "$OSTYPE" == "darwin"* ]]; then
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i '' 's/placement-policy.scheduling.x-k8s.io/placementpolicy.scheduling.x-k8s.io/g' {} +
else
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i 's/placement-policy.scheduling.x-k8s.io/placementpolicy.scheduling.x-k8s.io/g' {} +
fi

echo "Generating clientset at ${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}"
"${TOOLS_BIN_DIR}/client-gen" \
    --clientset-name "${CLIENTSET_NAME}" \
    --input-base "" \
    --input "${FQ_APIS}" \
    --output-package "${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

echo "Generating listers at ${OUTPUT_PKG}/listers"
"${TOOLS_BIN_DIR}/lister-gen" \
    --input-dirs "${FQ_APIS}" \
    --output-package "${OUTPUT_PKG}/listers" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

echo "Generating informers at ${OUTPUT_PKG}/informers"
"${TOOLS_BIN_DIR}/informer-gen" \
    --input-dirs "${FQ_APIS}" \
    --versioned-clientset-package "${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}/${CLIENTSET_NAME}" \
    --listers-package "${OUTPUT_PKG}/listers" \
    --output-package "${OUTPUT_PKG}/informers" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

echo "Generating register at ${FQ_APIS}"
"${TOOLS_BIN_DIR}/register-gen" \
    --input-dirs "${FQ_APIS}" \
    --output-package "${FQ_APIS}" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

# reference from https://github.com/servicemeshinterface/smi-sdk-go/blob/master/hack/update-codegen.sh
# replace placementpolicy.scheduling.x-k8s.io with placement-policy.scheduling.x-k8s.io after code generation
if [[ "$OSTYPE" == "darwin"* ]]; then
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i '' 's/placementpolicy.scheduling.x-k8s.io/placement-policy.scheduling.x-k8s.io/g' {} +
  find "${SCRIPT_ROOT}/pkg/client" -type f -exec sed -i '' 's/placementpolicy.scheduling.x-k8s.io/placement-policy.scheduling.x-k8s.io/g' {} +
else
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i 's/placementpolicy.scheduling.x-k8s.io/placement-policy.scheduling.x-k8s.io/g' {} +
  find "${SCRIPT_ROOT}/pkg/client" -type f -exec sed -i 's/placementpolicy.scheduling.x-k8s.io/placement-policy.scheduling.x-k8s.io/g' {} +
fi
