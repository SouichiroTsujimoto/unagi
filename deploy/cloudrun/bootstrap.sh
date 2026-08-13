#!/usr/bin/env bash
# One-time GCP + GitHub setup for Cloud Run deploys. Safe to re-run.
# Creates: APIs, Artifact Registry, runtime/deployer service accounts,
# Secret Manager entries (empty), Workload Identity Federation for GitHub
# Actions, and the repository variables the deploy workflow reads.
set -euo pipefail
# shellcheck source=deploy/cloudrun/config.sh
source "$(cd "$(dirname "$0")" && pwd)/config.sh"
cd "${root}"

gc=(gcloud --project="${GCP_PROJECT}" --quiet)
step() { printf '\n== %s\n' "$*"; }

step "enabling APIs"
"${gc[@]}" services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com

step "artifact registry: ${ARTIFACT_REPO} (${GCP_REGION})"
if ! "${gc[@]}" artifacts repositories describe "${ARTIFACT_REPO}" --location="${GCP_REGION}" >/dev/null 2>&1; then
  "${gc[@]}" artifacts repositories create "${ARTIFACT_REPO}" \
    --repository-format=docker --location="${GCP_REGION}" \
    --description="unagi container images"
fi
gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev" --quiet

step "service accounts"
create_sa() {
  local name="$1" title="$2" email
  # An address means the account is managed elsewhere.
  [[ "${name}" == *"@"* ]] && return 0
  email="$(sa_email "${name}")"
  if ! "${gc[@]}" iam service-accounts describe "${email}" >/dev/null 2>&1; then
    "${gc[@]}" iam service-accounts create "${name}" --display-name="${title}"
  fi
}
create_sa "${RUNTIME_SA}" "unagi Cloud Run runtime"
create_sa "${DEPLOYER_SA}" "unagi GitHub Actions deployer"

step "secrets"
for entry in "${secret_map[@]}"; do
  secret="${entry#*=}"
  if ! "${gc[@]}" secrets describe "${secret}" >/dev/null 2>&1; then
    "${gc[@]}" secrets create "${secret}" --replication-policy=automatic
    echo "created ${secret} (no version yet)"
  fi
  "${gc[@]}" secrets add-iam-policy-binding "${secret}" \
    --member="serviceAccount:${runtime_sa_email}" \
    --role=roles/secretmanager.secretAccessor >/dev/null
done

step "deployer roles"
for role in roles/run.admin roles/artifactregistry.writer; do
  "${gc[@]}" projects add-iam-policy-binding "${GCP_PROJECT}" \
    --member="serviceAccount:${deployer_sa_email}" --role="${role}" \
    --condition=None >/dev/null
done
# Needed so the deployer may run the service as the runtime account.
"${gc[@]}" iam service-accounts add-iam-policy-binding "${runtime_sa_email}" \
  --member="serviceAccount:${deployer_sa_email}" \
  --role=roles/iam.serviceAccountUser >/dev/null

if [[ -z "${GITHUB_REPO:-}" ]]; then
  echo
  echo "GITHUB_REPO is unset; skipping Workload Identity Federation."
  echo "Set GITHUB_REPO=owner/name in deploy/cloudrun/.env and re-run to enable CD."
  exit 0
fi

step "workload identity federation for ${GITHUB_REPO}"
if ! "${gc[@]}" iam workload-identity-pools describe "${WIF_POOL}" --location=global >/dev/null 2>&1; then
  "${gc[@]}" iam workload-identity-pools create "${WIF_POOL}" \
    --location=global --display-name="GitHub Actions"
fi
if ! "${gc[@]}" iam workload-identity-pools providers describe "${WIF_PROVIDER}" \
  --location=global --workload-identity-pool="${WIF_POOL}" >/dev/null 2>&1; then
  "${gc[@]}" iam workload-identity-pools providers create-oidc "${WIF_PROVIDER}" \
    --location=global --workload-identity-pool="${WIF_POOL}" \
    --display-name="GitHub Actions OIDC" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
    --attribute-condition="assertion.repository == '${GITHUB_REPO}'"
fi

pool_name="$("${gc[@]}" iam workload-identity-pools describe "${WIF_POOL}" \
  --location=global --format='value(name)')"
wif_provider="${pool_name}/providers/${WIF_PROVIDER}"

# Only this repository may impersonate the deployer account.
"${gc[@]}" iam service-accounts add-iam-policy-binding "${deployer_sa_email}" \
  --member="principalSet://iam.googleapis.com/${pool_name}/attribute.repository/${GITHUB_REPO}" \
  --role=roles/iam.workloadIdentityUser >/dev/null

step "github repository variables"
if ! command -v gh >/dev/null 2>&1; then
  echo "gh not found. Set these repository variables by hand:"
  echo "  GCP_WIF_PROVIDER=${wif_provider}"
  echo "  GCP_DEPLOY_SA=${deployer_sa_email}"
  exit 0
fi

set_var() {
  local name="$1" value="$2"
  [[ -z "${value}" ]] && return 0
  gh variable set "${name}" --repo "${GITHUB_REPO}" --body "${value}" >/dev/null
  echo "  ${name}"
}
set_var GCP_PROJECT "${GCP_PROJECT}"
set_var GCP_REGION "${GCP_REGION}"
set_var ARTIFACT_REPO "${ARTIFACT_REPO}"
set_var CLOUD_RUN_SERVICE "${CLOUD_RUN_SERVICE}"
set_var GCP_WIF_PROVIDER "${wif_provider}"
set_var GCP_DEPLOY_SA "${deployer_sa_email}"
set_var RUNTIME_SA "${runtime_sa_email}"
set_var UNIGO_SITE_BASE_URL "${UNIGO_SITE_BASE_URL:-}"
set_var UNIGO_MEDIA_PUBLIC_BASE "${UNIGO_MEDIA_PUBLIC_BASE:-}"
set_var UNIGO_SITE_NAME "${UNIGO_SITE_NAME:-}"
set_var UNIGO_SITE_DESCRIPTION "${UNIGO_SITE_DESCRIPTION:-}"
for entry in "${secret_map[@]}"; do
  set_var "${entry%%=*}_SECRET" "${entry#*=}"
done

printf '\ndone. next: fill secret values in deploy/cloudrun/.env, then just cloudrun-secret\n'
