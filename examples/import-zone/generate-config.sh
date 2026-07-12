#!/usr/bin/env bash
set -euo pipefail

BASE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROVIDER_DIR=${SUBREG_PROVIDER_DIR:-$(cd "${BASE_DIR}/../.." && pwd)}

DOMAIN=${TF_VAR_subreg_domain:-${SUBREG_DOMAIN:-}}
if [ -z "${DOMAIN}" ]; then
  echo "Set TF_VAR_subreg_domain or SUBREG_DOMAIN" >&2
  exit 1
fi

OUTPUT_FILE="${BASE_DIR}/generated_resources.tf"

TMP_FILE=$(mktemp)
rm -f "${TMP_FILE}"
trap 'rm -f "${TMP_FILE}"' EXIT

TF_CLI_CONFIG_FILE=terraform.rc terraform plan -input=false -generate-config-out="${TMP_FILE}"
cd "${PROVIDER_DIR}" && go run ./cmd/subreg-sort-config -input "${TMP_FILE}" -output "${OUTPUT_FILE}"

echo "Wrote ${OUTPUT_FILE}"
