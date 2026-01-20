#!/bin/bash
# Perform  build npu-exporter
# Copyright @ Huawei Technologies CO., Ltd. 2020-2023. All rights reserved
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
CUR_DIR=$(dirname $(readlink -f $0))
TOP_DIR=$(realpath "${CUR_DIR}"/..)
export GO111MODULE="on"
VER_FILE="${TOP_DIR}"/service_config.ini
build_version="v6.0.0"
if [ -f "$VER_FILE" ]; then
  line=$(sed -n '1p' "$VER_FILE" 2>&1)
  #cut the chars after ':' and add char 'v', the final example is v3.0.0
  build_version="v"${line#*=}
fi

arch=$(arch 2>&1)
echo "Build Architecture is" "${arch}"

OUTPUT_NAME="npu-exporter"
DOCKER_FILE_NAME="Dockerfile"
A200ISOC_DOCKER_FILE_NAME="Dockerfile-310P-1usoc"
A200ISOC_RUN_SHELL="run_for_310P_1usoc.sh"

function clean() {
  rm -rf "${TOP_DIR}"/output
  mkdir -p "${TOP_DIR}"/output
}

function build() {
  cd "${TOP_DIR}/cmd/npu-exporter"
  CGO_CFLAGS="-fstack-protector-strong -D_FORTIFY_SOURCE=2 -O2 -fPIC -ftrapv"
  CGO_CPPFLAGS="-fstack-protector-strong -D_FORTIFY_SOURCE=2 -O2 -fPIC -ftrapv"
  go build -mod=mod -buildmode=pie -ldflags "-s -extldflags=-Wl,-z,now  -X huawei.com/npu-exporter/v6/versions.BuildName=${OUTPUT_NAME} \
            -X huawei.com/npu-exporter/v6/versions.BuildVersion=${build_version}_linux-${arch}" \
    -o ${OUTPUT_NAME}
  ls ${OUTPUT_NAME}
  if [ $? -ne 0 ]; then
    echo "fail to find npu-exporter"
    exit 1
  fi
}

function mv_file() {
  mv "${TOP_DIR}"/cmd/npu-exporter/${OUTPUT_NAME} "${TOP_DIR}"/output
  cp "${TOP_DIR}"/build/npu-exporter.yaml "${TOP_DIR}"/output/npu-exporter-"${build_version}".yaml
  cp "${TOP_DIR}"/build/npu-exporter-310P-1usoc.yaml "${TOP_DIR}"/output/npu-exporter-310P-1usoc-"${build_version}".yaml
  cp "${TOP_DIR}"/build/metricConfiguration.json "${TOP_DIR}"/output/
  cp "${TOP_DIR}"/build/pluginConfiguration.json "${TOP_DIR}"/output/
  sed -i "s/npu-exporter:.*/npu-exporter:${build_version}/" "${TOP_DIR}"/output/npu-exporter-"${build_version}".yaml
  sed -i "s/npu-exporter:.*/npu-exporter:${build_version}/" "${TOP_DIR}"/output/npu-exporter-310P-1usoc-"${build_version}".yaml
  cp "${TOP_DIR}"/build/${DOCKER_FILE_NAME} "${TOP_DIR}"/output
  cp "${TOP_DIR}"/build/${A200ISOC_DOCKER_FILE_NAME} "${TOP_DIR}"/output
  cp "${TOP_DIR}"/build/${A200ISOC_RUN_SHELL} "${TOP_DIR}"/output
  chmod 400 "${TOP_DIR}"/output/*
  chmod 500 "${TOP_DIR}"/output/${OUTPUT_NAME}
  chmod 500 "${TOP_DIR}"/output/${A200ISOC_RUN_SHELL}

}

function main() {
  clean
  build
  mv_file
}

main
