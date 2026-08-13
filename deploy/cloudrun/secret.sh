#!/usr/bin/env bash
# Add Secret Manager versions from values in deploy/cloudrun/.env.
set -euo pipefail
# shellcheck source=deploy/cloudrun/config.sh
source "$(cd "$(dirname "$0")" && pwd)/config.sh"

if [[ ! -f "${env_file}" ]]; then
  echo "missing ${env_file}" >&2
  exit 1
fi

missing=()
for entry in "${secret_map[@]}"; do
  env_name="${entry%%=*}"
  if [[ -z "${!env_name:-}" ]]; then
    missing+=("${env_name}")
  fi
done
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "set these in deploy/cloudrun/.env:" >&2
  printf '  %s\n' "${missing[@]}" >&2
  exit 1
fi

for entry in "${secret_map[@]}"; do
  env_name="${entry%%=*}"
  secret="${entry#*=}"
  printf '%s' "${!env_name}" |
    gcloud secrets versions add "${secret}" --project="${GCP_PROJECT}" --data-file=- >/dev/null
  echo "added a version to ${secret}"
done
