#!/bin/bash
# Perform  test for  npu-exporter
# Copyright @ Huawei Technologies CO., Ltd. 2020-2020. All rights reserved
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# ============================================================================
set -e


# execute go test and echo result to report files
function execute_test() {
  if ! (go test -v -race -coverprofile cov.out "${TOP_DIR}"/... >./"$file_input")
  then
    echo '****** go test cases error! ******'
    cat $file_input
    exit 1
  else
    gocov convert cov.out | gocov-html >"$file_detail_output"
    gotestsum --junitfile unit-tests.xml "${TOP_DIR}"/...

    total_coverage=$(go tool cover -func=cov.out | grep "total:" | awk '{print $3}'| sed 's/%//')
    # round up
    coverage=$(echo "$total_coverage" | awk '{if ($1 >= 0) print ($1 == int($1)) ? int($1) : int($1) + 1;\
                                          else print ($1 == int($1)) ? int($1) : int($1)}')
    if [[ $coverage -ge 80 ]]; then
      echo "coverage passed: $coverage%"
      exit 0
    else
      echo "coverage failed: $coverage%, it needs to be greater than 80%."
      exit 1
    fi
  fi
}


export GO111MODULE="on"
export PATH=$GOPATH/bin:$PATH
export GOFLAGS="-gcflags=all=-l"
unset GOPATH
# if didn't install the following  tools, please install firstly
#go get -insecure github.com/axw/gocov/gocov
#go get github.com/matm/gocov-html
CUR_DIR=$(dirname "$(readlink -f "$0")")
TOP_DIR=$(realpath "${CUR_DIR}"/..)

file_input='testExporter.txt'
file_detail_output='api.html'

if [ -f "${TOP_DIR}"/test ]; then
  rm -rf "${TOP_DIR}"/test
fi
mkdir -p "${TOP_DIR}"/test
cd "${TOP_DIR}"/test
echo "clean old version test results"

if [ -f "$file_input" ]; then
  rm -rf "$file_input"
fi
if [ -f "$file_detail_output" ]; then
  rm -rf "$file_detail_output"
fi

echo "************************************* Start LLT Test *************************************"
execute_test
echo "************************************* End   LLT Test *************************************"
