#!/usr/bin/env bash
# Shared config for the Cloud Run scripts. Sourced, not executed.
# Values come from deploy/cloudrun/.env when present, else the shell env.
# shellcheck disable=SC2034  # consumed by the sourcing script

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

env_file="${root}/deploy/cloudrun/.env"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${env_file}"
  set +a
fi

: "${GCP_PROJECT:?set GCP_PROJECT (see deploy/cloudrun/env.example)}"
: "${GCP_REGION:=asia-northeast1}"
: "${ARTIFACT_REPO:=unagi}"
: "${CLOUD_RUN_SERVICE:=unagi}"
: "${RUNTIME_SA:=unagi-run}"
: "${DEPLOYER_SA:=unagi-deployer}"
: "${WIF_POOL:=github}"
: "${WIF_PROVIDER:=github}"

: "${UNIGO_DB_DSN_SECRET:=unagi-db-dsn}"
: "${UNIGO_SUPABASE_URL_SECRET:=unagi-supabase-url}"
: "${UNIGO_SUPABASE_PUBLISHABLE_KEY_SECRET:=unagi-supabase-publishable}"
: "${UNIGO_SUPABASE_SECRET_KEY_SECRET:=unagi-supabase-secret}"
: "${UNIGO_ADMIN_USER_IDS_SECRET:=unagi-admin-ids}"

# env var name → Secret Manager secret name.
secret_map=(
  "UNIGO_DB_DSN=${UNIGO_DB_DSN_SECRET}"
  "UNIGO_SUPABASE_URL=${UNIGO_SUPABASE_URL_SECRET}"
  "UNIGO_SUPABASE_PUBLISHABLE_KEY=${UNIGO_SUPABASE_PUBLISHABLE_KEY_SECRET}"
  "UNIGO_SUPABASE_SECRET_KEY=${UNIGO_SUPABASE_SECRET_KEY_SECRET}"
  "UNIGO_ADMIN_USER_IDS=${UNIGO_ADMIN_USER_IDS_SECRET}"
)

image_repo="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT}/${ARTIFACT_REPO}/${CLOUD_RUN_SERVICE}"

sa_email() {
  local name="$1"
  if [[ "${name}" == *"@"* ]]; then
    printf '%s' "${name}"
  else
    printf '%s@%s.iam.gserviceaccount.com' "${name}" "${GCP_PROJECT}"
  fi
}

runtime_sa_email="$(sa_email "${RUNTIME_SA}")"
deployer_sa_email="$(sa_email "${DEPLOYER_SA}")"
