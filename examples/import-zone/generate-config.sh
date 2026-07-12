#!/usr/bin/env bash
set -euo pipefail

BASE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROVIDER_DIR=${SUBREG_PROVIDER_DIR:-$(cd "${BASE_DIR}/../.." && pwd)}

DOMAIN=${TF_VAR_subreg_domain:-${SUBREG_DOMAIN:-}}
if [ -z "${DOMAIN}" ]; then
  echo "Set TF_VAR_subreg_domain or SUBREG_DOMAIN" >&2
  exit 1
fi

FIRST_CHAR=${DOMAIN:0:1}
FIRST_CHAR=${FIRST_CHAR,,}
case "${FIRST_CHAR}" in
  [a-z0-9]) ;;
  *) FIRST_CHAR="_" ;;
esac

OUTPUT_DIR="${BASE_DIR}/domains/${FIRST_CHAR}"
OUTPUT_FILE="${OUTPUT_DIR}/${DOMAIN}.tf"

TMP_FILE=$(mktemp)
rm -f "${TMP_FILE}"
trap 'rm -f "${TMP_FILE}"' EXIT

mkdir -p "${OUTPUT_DIR}"

TF_CLI_CONFIG_FILE=terraform.rc terraform plan -input=false -generate-config-out="${TMP_FILE}"
cd "${PROVIDER_DIR}" && go run ./cmd/subreg-sort-config -input "${TMP_FILE}" -output "${OUTPUT_FILE}"

echo "Wrote ${OUTPUT_FILE}"
