#!/usr/bin/env bash
set -euo pipefail

BASE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${BASE_DIR}/../.." && pwd)

DOMAIN=${TF_VAR_subreg_domain:-${SUBREG_DOMAIN:-}}
if [ -z "${DOMAIN}" ]; then
  echo "Set TF_VAR_subreg_domain or SUBREG_DOMAIN" >&2
  exit 1
fi

cd "${ROOT_DIR}"
go run ./cmd/subreg-import -domain "${DOMAIN}" > "${BASE_DIR}/imports.tf"

echo "Wrote ${BASE_DIR}/imports.tf"
