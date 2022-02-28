#!/usr/bin/env bash

# Copyright 2022 The OpenYurt Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail


BASE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELM_DIR=${BASE_ROOT}/charts/yurtcluster-operator
_tmpdir=/tmp/yurtcluster-operator

function verify:package:helm:files {
    mkdir -p ${_tmpdir}
    cd $HELM_DIR && helm package . -d ${_tmpdir} > /dev/null && mv ${_tmpdir}/*.tgz ${_tmpdir}/yurtcluster-operator.tgz
    helm_checksum=$(tar -xOzf ${HELM_DIR}/../yurtcluster-operator.tgz | sort | sha1sum | awk '{ print $1 }')
    temp_helm_checksum=$(tar -xOzf ${_tmpdir}/yurtcluster-operator.tgz | sort | sha1sum | awk '{ print $1 }')
    if [ "$helm_checksum" != "$temp_helm_checksum" ]; then
      echo "Helm package yurtcluster-operator.tgz not updated or the helm chart is not expected."
      echo "Please run:  make update-helm-package"
      exit 1
    fi
}

function cleanup {
  #echo "Removing workspace: ${_tmpdir}"
  rm -rf "${_tmpdir}"
}

trap "cleanup" EXIT SIGINT

verify:package:helm:files
cleanup