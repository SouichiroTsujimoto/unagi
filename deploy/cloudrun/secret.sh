#!/usr/bin/env bash
# Add a new version to a Secret Manager secret. The value is read from stdin
# (prompted, not echoed) so it never lands in shell history or a file.
set -euo pipefail
# shellcheck source=deploy/cloudrun/config.sh
source "$(cd "$(dirname "$0")" && pwd)/config.sh"

secret="${1:-}"
if [[ -z "${secret}" ]]; then
  echo "usage: just cloudrun-secret <secret-name>" >&2
  echo "secrets:" >&2
  for entry in "${secret_map[@]}"; do
    printf '  %-34s %s\n' "${entry#*=}" "${entry%%=*}" >&2
  done
  exit 2
fi

if [[ -t 0 ]]; then
  printf 'value for %s (input hidden): ' "${secret}" >&2
  IFS= read -rs value
  printf '\n' >&2
else
  IFS= read -r value
fi
[[ -z "${value}" ]] && {
  echo "empty value" >&2
  exit 1
}

printf '%s' "${value}" |
  gcloud secrets versions add "${secret}" --project="${GCP_PROJECT}" --data-file=- >/dev/null
echo "added a version to ${secret}"
