#!/usr/bin/env bash

# Copyright 2021 The OpenYurt Authors.
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

YURT_ROOT=$(dirname "${BASH_SOURCE[0]}")/../../

yurt::version::get_version_vars() {
  # Prepare git command
  local git=(git --work-tree "${YURT_ROOT}")

  if YURT_GIT_COMMIT=$("${git[@]}" rev-parse "HEAD^{commit}" 2>/dev/null); then
    # Use git describe to find the version based on tags.
    if YURT_GIT_VERSION=$("${git[@]}" describe --tags --match='v*' --abbrev=14 "${YURT_GIT_COMMIT}^{commit}" 2>/dev/null); then

      # Try to match the "git describe" output to a regex to try to extract
      # the "major" and "minor" versions and whether this is the exact tagged
      # version or whether the tree is between two tagged versions.
      if [[ "${YURT_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)?([-].*)?([+].*)?$ ]]; then
        YURT_GIT_MAJOR=${BASH_REMATCH[1]}
        YURT_GIT_MINOR=${BASH_REMATCH[2]}
        if [[ -n "${BASH_REMATCH[4]}" ]]; then
          YURT_GIT_MINOR+="+"
        fi
      fi

      # If YURT_GIT_VERSION is not a valid Semantic Version, then refuse to build.
      if ! [[ "${YURT_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)?(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
          echo "YURT_GIT_VERSION should be a valid Semantic Version. Current value: ${YURT_GIT_VERSION}"
          echo "Please see more details here: https://semver.org"
          exit 1
      fi

    fi
  fi
}

# Prints the value that needs to be passed to the -ldflags parameter of go build
yurt::version::ldflags() {
  yurt::version::get_version_vars

  local -a ldflags
  function add_ldflag() {
    local key=${1}
    local val=${2}
    ldflags+=(
      "-X 'github.com/openyurtio/yurtcluster-operator/pkg/version.${key}=${val}'"
    )
  }

  add_ldflag "buildDate" "$(date ${SOURCE_DATE_EPOCH:+"--date=@${SOURCE_DATE_EPOCH}"} -u +'%Y-%m-%dT%H:%M:%SZ')"
  if [[ -n ${YURT_GIT_COMMIT-} ]]; then
    add_ldflag "gitCommit" "${YURT_GIT_COMMIT}"
  fi

  if [[ -n ${YURT_GIT_VERSION-} ]]; then
    add_ldflag "gitVersion" "${YURT_GIT_VERSION}"
  fi

  if [[ -n ${YURT_GIT_MAJOR-} && -n ${YURT_GIT_MINOR-} ]]; then
    add_ldflag "gitMajor" "${YURT_GIT_MAJOR}"
    add_ldflag "gitMinor" "${YURT_GIT_MINOR}"
  fi

  # The -ldflags parameter takes a single string, so join the output.
  echo "${ldflags[*]-}"
}

"$@"
