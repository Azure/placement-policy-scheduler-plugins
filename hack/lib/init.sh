#!/usr/bin/env bash

unset CDPATH

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/../..

source "${SCRIPT_ROOT}/hack/lib/logging.sh"
source "${SCRIPT_ROOT}/hack/lib/etcd.sh"
source "${SCRIPT_ROOT}/hack/lib/util.sh"
source "${SCRIPT_ROOT}/hack/lib/golang.sh"