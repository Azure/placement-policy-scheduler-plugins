#!/usr/bin/env bash
#https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/hack/install-etcd.sh

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SCRIPT_ROOT}/hack/lib/init.sh"

kube::etcd::install
