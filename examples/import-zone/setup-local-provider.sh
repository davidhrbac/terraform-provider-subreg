#!/usr/bin/env bash
set -euo pipefail

VERSION=${VERSION:-0.1.0}

BASE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${BASE_DIR}/../.." && pwd)

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
PLATFORM="${GOOS}_${GOARCH}"

PLUGIN_DIR="${BASE_DIR}/plugins"
TARGET_DIR="${PLUGIN_DIR}/registry.terraform.io/davidhrbac/subreg/${VERSION}/${PLATFORM}"

mkdir -p "${TARGET_DIR}"

BINARY="terraform-provider-subreg_v${VERSION}"
if [ "${GOOS}" = "windows" ]; then
  BINARY="${BINARY}.exe"
fi

go build -o "${TARGET_DIR}/${BINARY}" -ldflags "-X main.version=${VERSION}" "${ROOT_DIR}"

cat > "${BASE_DIR}/terraform.rc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "${PLUGIN_DIR}"
    include = ["davidhrbac/subreg"]
  }
}
EOF

echo "Wrote ${BASE_DIR}/terraform.rc"
echo "Built ${TARGET_DIR}/${BINARY}"

if [ "${SKIP_TF_CLEAN:-}" != "1" ]; then
  rm -f "${BASE_DIR}/imports.tf" "${BASE_DIR}/generated_resources.tf"
  rm -f "${BASE_DIR}/.terraform.lock.hcl"
  rm -rf "${BASE_DIR}/.terraform"
  echo "Removed ${BASE_DIR}/imports.tf and ${BASE_DIR}/generated_resources.tf"
  echo "Removed ${BASE_DIR}/.terraform.lock.hcl and ${BASE_DIR}/.terraform"
  echo "Run: TF_CLI_CONFIG_FILE=terraform.rc terraform init"
fi
