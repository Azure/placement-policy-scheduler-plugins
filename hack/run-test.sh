#!/usr/bin/env bash
#https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/hack/integration-test.sh

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${SCRIPT_ROOT}/hack/lib/init.sh"

checkEtcdOnPath() {
  export PATH="$(pwd)/etcd:${PATH}"
  kube::log::status "Checking etcd is on PATH"
  command -v etcd >/dev/null && return
  kube::log::status "Cannot find etcd, cannot run integration tests."
  kube::log::status "Please see https://git.k8s.io/community/contributors/devel/sig-testing/integration-tests.md#install-etcd-dependency for instructions."
  # kube::log::usage "You can use 'hack/install-etcd.sh' to install a copy in third_party/."
  retucrn 1
}

CLEANUP_REQUIRED=
cleanup() {
  [[ -z "${CLEANUP_REQUIRED}" ]] && return
  kube::log::status "Cleaning up etcd"
  kube::etcd::cleanup
  CLEANUP_REQUIRED=
  kube::log::status "Integration test cleanup complete"
}

runTests() {
  kube::log::status "Starting etcd instance"
  CLEANUP_REQUIRED=1
  kube::etcd::start
  kube::log::status "Running integration test cases"

  go test ./... -mod=vendor -coverprofile cover.out

  cleanup
}

checkEtcdOnPath

# Run cleanup to stop etcd on interrupt or other kill signal.
trap cleanup EXIT

runTests
