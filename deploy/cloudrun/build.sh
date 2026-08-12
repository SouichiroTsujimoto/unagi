#!/usr/bin/env bash
# Build the linux/amd64 distroless image and push it to Artifact Registry.
set -euo pipefail
# shellcheck source=deploy/cloudrun/config.sh
source "$(cd "$(dirname "$0")" && pwd)/config.sh"
cd "${root}"

version="${IMAGE_TAG:-}"
if [[ -z "${version}" ]]; then
  version="$(git describe --tags --always --dirty 2>/dev/null || date -u +%Y%m%d%H%M%S)"
fi

image="${image_repo}:${version}"
echo "building ${image}"
docker build \
  --platform=linux/amd64 \
  --build-arg "VERSION=${version}" \
  -t "${image}" \
  -t "${image_repo}:latest" \
  .
echo "pushing ${image}"
docker push "${image}"
docker push "${image_repo}:latest"
echo "IMAGE=${image}"
