#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/build_proxyvm.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

source ./scripts/constants.sh

echo "Building proxyvm plugin..."
go build -o ./build/proxyvm ./vms/example/proxyvm/cmd/proxyvm/

PLUGIN_DIR="$HOME/.avalanchego/plugins"
PLUGIN_PATH="${PLUGIN_DIR}/rXJsEqRDAFw7svqnNME1rJgi1B5fSVJ28mvgJAUJg7nDv8WYo"
echo "Symlinking ./build/proxyvm to ${PLUGIN_PATH}"
mkdir -p "${PLUGIN_DIR}"
ln -sf "${PWD}/build/proxyvm" "${PLUGIN_PATH}"
