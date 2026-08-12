#!/usr/bin/env bash
# Deploy an image to Cloud Run (Tokyo, min-instances=0).
# IMAGE overrides the tag; the default is :latest from Artifact Registry.
set -euo pipefail
# shellcheck source=deploy/cloudrun/config.sh
source "$(cd "$(dirname "$0")" && pwd)/config.sh"
cd "${root}"

: "${UNIGO_SITE_BASE_URL:?set UNIGO_SITE_BASE_URL}"
: "${UNIGO_MEDIA_PUBLIC_BASE:?set UNIGO_MEDIA_PUBLIC_BASE}"

image="${IMAGE:-${image_repo}:latest}"

env_vars=(
  "UNIGO_SITE_BASE_URL=${UNIGO_SITE_BASE_URL}"
  "UNIGO_MEDIA_PUBLIC_BASE=${UNIGO_MEDIA_PUBLIC_BASE}"
  "UNIGO_MEDIA_BACKEND=supabase"
)
[[ -n "${UNIGO_SITE_NAME:-}" ]] && env_vars+=("UNIGO_SITE_NAME=${UNIGO_SITE_NAME}")
[[ -n "${UNIGO_SITE_DESCRIPTION:-}" ]] && env_vars+=("UNIGO_SITE_DESCRIPTION=${UNIGO_SITE_DESCRIPTION}")

# Secrets stay in Secret Manager; only their names travel through here.
secret_args=()
for entry in "${secret_map[@]}"; do
  secret_args+=("${entry%%=*}=${entry#*=}:latest")
done

deploy=(
  gcloud run deploy "${CLOUD_RUN_SERVICE}"
  --project="${GCP_PROJECT}"
  --region="${GCP_REGION}"
  --image="${image}"
  --platform=managed
  --service-account="${runtime_sa_email}"
  --allow-unauthenticated
  --min-instances=0
  --max-instances=3
  --memory=512Mi
  --cpu=1
  --port=8080
  --set-env-vars="$(
    IFS=,
    echo "${env_vars[*]}"
  )"
  --set-secrets="$(
    IFS=,
    echo "${secret_args[*]}"
  )"
)

echo "deploying ${image} → ${CLOUD_RUN_SERVICE} (${GCP_REGION})"
"${deploy[@]}"
